# OpenAPI Specifications

This directory contains OpenAPI/Swagger specifications for Maestro APIs.

## Files

### `rest-api.yaml`
OpenAPI 3.0 specification for the REST management API. Defines all endpoints for:
- Cluster management
- Agent management
- Task management (CRUD, reorder, reset executions)
- Execution viewing
- Debug task management
- Health checks

**Code Generation:** Run `make generate-api` to generate Go server interfaces and types from this spec.

**Generated Output:** `internal/api/rest/openapi/generated.go`

### `agent.swagger.json`
Swagger 2.0 specification for the gRPC agent API (auto-generated from proto files).

**Generated From:** `api/proto/agent/v1/agent.proto`

**Generation:** Run `make proto` to regenerate.

## Using Generated Code

The REST API code is generated using `oapi-codegen` from `rest-api.yaml`.

### What Gets Generated

1. **Type Definitions** - All request/response models
   ```go
   type Cluster struct {
       Id          openapi_types.UUID `json:"id"`
       Name        string             `json:"name"`
       Description *string            `json:"description,omitempty"`
       CreatedAt   time.Time          `json:"created_at"`
       UpdatedAt   time.Time          `json:"updated_at"`
   }
   ```

2. **Server Interface** - Methods your handlers must implement
   ```go
   type ServerInterface interface {
       ListClusters(w http.ResponseWriter, r *http.Request, params ListClustersParams)
       CreateCluster(w http.ResponseWriter, r *http.Request)
       GetCluster(w http.ResponseWriter, r *http.Request, id ID)
       DeleteCluster(w http.ResponseWriter, r *http.Request, id ID)
       // ... all other endpoints
   }
   ```

3. **Chi Router Setup** - Function to wire up routes
   ```go
   func HandlerFromMux(si ServerInterface, r chi.Router) http.Handler
   ```

4. **Embedded Spec** - The OpenAPI spec embedded in the binary for runtime use

### Implementation Pattern

**Option 1: Implement the Interface Directly**

```go
package rest

import "github.com/filanov/maestro/internal/api/rest/openapi"

type Server struct {
    db db.DB
}

// Implement all ServerInterface methods
func (s *Server) ListClusters(w http.ResponseWriter, r *http.Request, params openapi.ListClustersParams) {
    limit := 50
    if params.Limit != nil {
        limit = *params.Limit
    }

    clusters, total, err := s.db.ListClusters(r.Context(), limit, offset)
    // ... handle response
}
```

**Option 2: Adapter Pattern** (keeps existing handlers)

```go
package rest

type OpenAPIAdapter struct {
    server *Server  // Your existing REST server
}

func (a *OpenAPIAdapter) ListClusters(w http.ResponseWriter, r *http.Request, params openapi.ListClustersParams) {
    // Adapt params to existing handler format
    a.server.handleListClusters(w, r)
}
```

### Regenerating Code

When you modify `rest-api.yaml`:

```bash
# Regenerate REST API code
make generate-api

# Regenerate everything (proto + REST API)
make generate
```

## Benefits of Generated Code

✅ **Type Safety** - Compile-time checks for request/response structures
✅ **Automatic Validation** - Request validation based on OpenAPI constraints
✅ **API Documentation** - Single source of truth (OpenAPI spec)
✅ **Contract-Driven** - API design before implementation
✅ **Client Generation** - Can generate clients in multiple languages
✅ **Testing** - Easier to mock and test with defined interfaces

## Viewing the API Documentation

You can use various tools to view/interact with the OpenAPI spec:

### Swagger UI
```bash
docker run -p 8081:8080 -e SWAGGER_JSON=/api/rest-api.yaml \
  -v $(pwd)/api/openapi:/api swaggerapi/swagger-ui
```
Open: http://localhost:8081

### Redoc
```bash
docker run -p 8081:80 -e SPEC_URL=/api/rest-api.yaml \
  -v $(pwd)/api/openapi:/usr/share/nginx/html/api redocly/redoc
```
Open: http://localhost:8081

### VSCode Extension
Install "OpenAPI (Swagger) Editor" extension and open `rest-api.yaml`.

## Configuration

Generation is configured in `.oapi-codegen.yaml`:

```yaml
package: openapi
output: internal/api/rest/openapi/generated.go
generate:
  models: true         # Generate type definitions
  chi-server: true     # Generate chi router integration
  strict-server: false # Use standard server interface
  embedded-spec: true  # Embed OpenAPI spec in generated code
```

## Next Steps

To fully adopt generated code:

1. **Update Handlers** - Implement `openapi.ServerInterface` in your server
2. **Use Generated Types** - Replace DTOs with generated types
3. **Wire Up Router** - Use `openapi.HandlerFromMux` to set up routes
4. **Remove Old Code** - Phase out handwritten router/handlers once migrated
5. **Add Validation** - Leverage built-in request validation

Or keep both approaches:
- Generated code for documentation and client generation
- Existing handlers for actual implementation
- Bridge with adapter pattern
