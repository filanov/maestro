package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/filanov/maestro/internal/config"
	"github.com/filanov/maestro/internal/models"
	pb "github.com/filanov/maestro/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const (
	MaxOutputSize       = 1024 * 1024
	DefaultTimeout      = 30 * time.Minute
	MaxTimeout          = 24 * time.Hour
	MinTimeout          = 10 * time.Second
	DebugTimeout        = 5 * time.Minute
	ShutdownGracePeriod = 30 * time.Second
)

type Agent struct {
	client            pb.AgentServiceClient
	agentID           string
	heartbeatInterval time.Duration
	taskPollInterval  time.Duration
	debugPollInterval time.Duration
}

func main() {
	if err := run(); err != nil {
		slog.Error("agent failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	setupLogging(cfg.LogLevel, cfg.LogFormat)

	hostname := cfg.Hostname
	if hostname == "auto" {
		hostname, err = os.Hostname()
		if err != nil {
			return fmt.Errorf("get hostname: %w", err)
		}
	}

	agentID := models.GenerateAgentID(cfg.ClusterID, hostname)

	slog.Info("starting maestro agent",
		"agent_id", agentID,
		"cluster_id", cfg.ClusterID,
		"hostname", hostname,
	)

	conn, err := connectWithRetry(cfg.ServiceHost, cfg.ServicePort)
	if err != nil {
		return fmt.Errorf("connect to service: %w", err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)

	resp, err := client.Register(context.Background(), &pb.RegisterRequest{
		AgentId:   agentID,
		ClusterId: cfg.ClusterID,
		Hostname:  hostname,
	})
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	if resp.GetReset_() {
		slog.Warn("re-registration detected, all tasks will be re-executed")
	}

	agent := &Agent{
		client:            client,
		agentID:           agentID,
		heartbeatInterval: cfg.HeartbeatInterval,
		taskPollInterval:  cfg.TaskPollInterval,
		debugPollInterval: cfg.DebugPollInterval,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go agent.heartbeatLoop(ctx)
	go agent.taskLoop(ctx)
	go agent.debugLoop(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	slog.Info("shutdown signal received, stopping new work")
	cancel()

	time.Sleep(ShutdownGracePeriod)
	slog.Info("agent shutdown complete")

	return nil
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := a.client.Heartbeat(ctx, &pb.HeartbeatRequest{
				AgentId: a.agentID,
			})
			if err != nil {
				if status.Code(err) == codes.NotFound {
					slog.Error("cluster deleted, exiting agent")
					os.Exit(1)
				}
				slog.Warn("heartbeat failed", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *Agent) taskLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("task loop stopping")
			return
		default:
			resp, err := a.client.PollTasks(ctx, &pb.PollTasksRequest{
				AgentId: a.agentID,
			})
			if err != nil {
				slog.Warn("poll tasks failed", "error", err)
				time.Sleep(a.taskPollInterval)
				continue
			}

			if len(resp.Tasks) == 0 {
				time.Sleep(a.taskPollInterval)
				continue
			}

			for _, task := range resp.Tasks {
				if ctx.Err() != nil {
					return
				}
				a.executeTask(ctx, task)
			}
		}
	}
}

func (a *Agent) executeTask(ctx context.Context, task *pb.Task) {
	slog.Info("executing task", "task_id", task.Id, "task_name", task.Name)

	var timeout time.Duration
	var command, workDir string

	switch cfg := task.Config.(type) {
	case *pb.Task_Exec:
		timeout = time.Duration(cfg.Exec.TimeoutSeconds) * time.Second
		command = cfg.Exec.Command
		workDir = cfg.Exec.WorkingDir
	default:
		slog.Error("unsupported task type", "task_id", task.Id)
		return
	}

	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if timeout < MinTimeout {
		timeout = MinTimeout
	}
	if timeout > MaxTimeout {
		timeout = MaxTimeout
	}

	a.reportExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_RUNNING, "", 0, "")

	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		slog.Error("working directory does not exist", "path", workDir)
		a.reportExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_FAILED, "", -1,
			fmt.Sprintf("Working directory does not exist: %s", workDir))
		return
	}

	execCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", command)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	outputStr := sanitizeOutput(output)
	outputStr = truncateOutput(outputStr)

	if execCtx.Err() == context.DeadlineExceeded {
		slog.Warn("task timeout exceeded", "task_id", task.Id)
		a.reportExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_FAILED,
			outputStr, -1, "Task timeout exceeded")
		return
	}

	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		slog.Warn("task failed", "task_id", task.Id, "exit_code", exitCode)
		a.reportExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_FAILED,
			outputStr, int32(exitCode), err.Error())
		return
	}

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	slog.Info("task completed successfully", "task_id", task.Id)
	a.reportExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS,
		outputStr, int32(exitCode), "")
}

func (a *Agent) debugLoop(ctx context.Context) {
	ticker := time.NewTicker(a.debugPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			resp, err := a.client.PollDebugTasks(ctx, &pb.PollDebugTasksRequest{
				AgentId: a.agentID,
			})
			if err != nil {
				slog.Warn("poll debug tasks failed", "error", err)
				continue
			}

			for _, debugTask := range resp.DebugTasks {
				a.executeDebugTask(ctx, debugTask)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *Agent) executeDebugTask(ctx context.Context, task *pb.DebugTask) {
	slog.Info("executing debug task", "debug_task_id", task.Id)

	timeout := time.Duration(task.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = DebugTimeout
	}

	execCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", task.Command)
	output, err := cmd.CombinedOutput()

	outputStr := sanitizeOutput(output)
	outputStr = truncateOutput(outputStr)

	if execCtx.Err() == context.DeadlineExceeded {
		a.reportDebugExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_FAILED,
			outputStr, -1, "Debug task timeout exceeded")
		return
	}

	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		a.reportDebugExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_FAILED,
			outputStr, int32(exitCode), err.Error())
		return
	}

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	a.reportDebugExecutionWithRetry(task.Id, pb.ExecutionStatus_EXECUTION_STATUS_SUCCESS,
		outputStr, int32(exitCode), "")
}

func (a *Agent) reportExecutionWithRetry(taskID string, execStatus pb.ExecutionStatus,
	output string, exitCode int32, errorMsg string) {

	req := &pb.ReportTaskExecutionRequest{
		AgentId:  a.agentID,
		TaskId:   taskID,
		Status:   execStatus,
		Output:   output,
		ExitCode: exitCode,
		Error:    errorMsg,
	}

	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := a.client.ReportTaskExecution(context.Background(), req)
		if err == nil {
			return
		}

		code := status.Code(err)
		if code == codes.NotFound || code == codes.InvalidArgument {
			slog.Error("failed to report execution", "error", err)
			return
		}

		slog.Warn("failed to report execution, retrying", "attempt", attempt, "error", err)
		time.Sleep(backoff)
		backoff *= 2
	}

	slog.Error("failed to report execution after retries")
}

func (a *Agent) reportDebugExecutionWithRetry(debugTaskID string, execStatus pb.ExecutionStatus,
	output string, exitCode int32, errorMsg string) {

	req := &pb.ReportDebugTaskExecutionRequest{
		AgentId:     a.agentID,
		DebugTaskId: debugTaskID,
		Status:      execStatus,
		Output:      output,
		ExitCode:    exitCode,
		Error:       errorMsg,
	}

	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := a.client.ReportDebugTaskExecution(context.Background(), req)
		if err == nil {
			return
		}

		code := status.Code(err)
		if code == codes.NotFound || code == codes.InvalidArgument {
			slog.Error("failed to report debug execution", "error", err)
			return
		}

		slog.Warn("failed to report debug execution, retrying", "attempt", attempt, "error", err)
		time.Sleep(backoff)
		backoff *= 2
	}

	slog.Error("failed to report debug execution after retries")
}

func sanitizeOutput(output []byte) string {
	if utf8.Valid(output) {
		return string(output)
	}
	return strings.ToValidUTF8(string(output), "�")
}

func truncateOutput(output string) string {
	if len(output) <= MaxOutputSize {
		return output
	}
	truncated := output[len(output)-MaxOutputSize:]
	return "... (output truncated) ...\n" + truncated
}

func connectWithRetry(host string, port int) (*grpc.ClientConn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	var conn *grpc.ClientConn
	var err error

	backoff := time.Second
	for attempt := 1; attempt <= 5; attempt++ {
		conn, err = grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                10 * time.Second,
				Timeout:             3 * time.Second,
				PermitWithoutStream: true,
			}),
		)
		if err == nil {
			return conn, nil
		}

		slog.Warn("failed to connect, retrying", "attempt", attempt, "error", err)
		time.Sleep(backoff)
		backoff *= 2
	}

	return nil, fmt.Errorf("failed to connect after 5 attempts: %w", err)
}

func setupLogging(level, format string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	slog.SetDefault(slog.New(handler))
}
