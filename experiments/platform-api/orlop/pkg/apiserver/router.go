package apiserver

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/thetechnick/orlop/pkg/apiserver/conversion"
	"github.com/thetechnick/orlop/pkg/apiserver/handlers"
	"github.com/thetechnick/orlop/pkg/apiserver/middleware"
	"k8s.io/apimachinery/pkg/runtime"
)

// setupRouter configures the HTTP router with all endpoints.
func setupRouter(registry *ResourceRegistry, corsOrigins []string, customMiddleware []func(http.Handler) http.Handler) (chi.Router, error) {
	r := chi.NewRouter()

	// Add CORS middleware
	r.Use(middleware.CORS(middleware.CORSOptions{
		AllowedOrigins: corsOrigins,
	}))

	for _, mw := range customMiddleware {
		r.Use(mw)
	}

	// Create discovery handler
	discoveryHandler := handlers.NewDiscoveryHandler(registry)

	// Discovery endpoints (must be registered BEFORE resource routes to avoid shadowing)
	r.Get("/apis", discoveryHandler.APIGroupList)
	r.Get("/openapi/v2", discoveryHandler.OpenAPIV2)
	r.Get("/openapi/v3", discoveryHandler.OpenAPIV3)

	// Group resources by GroupVersion
	gvResources := make(map[string][]ResourceInfo)
	for _, res := range registry.GetResources() {
		gv := fmt.Sprintf("%s/%s", res.GVK.Group, res.GVK.Version)
		gvResources[gv] = append(gvResources[gv], res)
	}

	// Setup routes for each GroupVersion
	for gv, resources := range gvResources {
		group := resources[0].GVK.Group
		version := resources[0].GVK.Version
		apiPath := "/apis/" + gv

		r.Route(apiPath, func(r chi.Router) {
			// Discovery endpoint for this specific group/version (before namespaced routes)
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				discoveryHandler.APIResourceList(w, req, group, version)
			})

			// Cluster-scoped resources get CRUD directly under the GV path
			for _, res := range resources {
				if res.Namespaced {
					continue
				}
				handler, err := registry.CreateHandler(res)
				if err != nil {
					continue
				}
				plural := res.Plural
				r.Post("/"+plural, handler.Create)
				r.Get("/"+plural, handler.List)
				r.Get("/"+plural+"/{name}", handler.Get)
				r.Put("/"+plural+"/{name}", handler.Update)
				r.Patch("/"+plural+"/{name}", handler.Patch)
				r.Delete("/"+plural+"/{name}", handler.Delete)
				r.Put("/"+plural+"/{name}/status", handler.UpdateStatus)
			}

			// Namespaced resources: LIST across all namespaces + CRUD under /namespaces/{namespace}
			var namespacedHandlers []struct {
				res     ResourceInfo
				handler *handlers.ResourceHandler
			}
			for _, res := range resources {
				if !res.Namespaced {
					continue
				}
				handler, err := registry.CreateHandler(res)
				if err != nil {
					continue
				}
				r.Get("/"+res.Plural, handler.List)
				namespacedHandlers = append(namespacedHandlers, struct {
					res     ResourceInfo
					handler *handlers.ResourceHandler
				}{res, handler})
			}
			if len(namespacedHandlers) > 0 {
				r.Route("/namespaces/{namespace}", func(r chi.Router) {
					for _, nh := range namespacedHandlers {
						plural := nh.res.Plural
						r.Post("/"+plural, nh.handler.Create)
						r.Get("/"+plural, nh.handler.List)
						r.Get("/"+plural+"/{name}", nh.handler.Get)
						r.Put("/"+plural+"/{name}", nh.handler.Update)
						r.Patch("/"+plural+"/{name}", nh.handler.Patch)
						r.Delete("/"+plural+"/{name}", nh.handler.Delete)
						r.Put("/"+plural+"/{name}/status", nh.handler.UpdateStatus)

						// Nested routes under parent resource (e.g. /clusters/{clusterID}/nodepools)
						if p := nh.res.ParentResource; p != nil {
							idField := p.IDField
							r.Route("/"+p.Plural+"/{parentID}/"+plural, func(r chi.Router) {
								r.Use(parentFilterMiddleware(idField, "parentID"))
								r.Post("/", nh.handler.Create)
								r.Get("/", nh.handler.List)
								r.Get("/{name}", nh.handler.Get)
								r.Put("/{name}", nh.handler.Update)
								r.Patch("/{name}", nh.handler.Patch)
								r.Delete("/{name}", nh.handler.Delete)
								r.Put("/{name}/status", nh.handler.UpdateStatus)
							})
						}
					}
				})
			}
		})

		// Per-group discovery endpoint
		r.Get("/apis/"+group, func(w http.ResponseWriter, req *http.Request) {
			discoveryHandler.APIGroup(w, req, group)
		})

		// OpenAPI v3 per-group-version endpoint
		r.Get("/openapi/v3/apis/"+gv, func(w http.ResponseWriter, req *http.Request) {
			discoveryHandler.OpenAPIV3GroupVersion(w, req, group, version)
		})
	}

	return r, nil
}

// parentFilterMiddleware returns a chi middleware that injects a ParentFilter into the
// request context. idField is the dot-separated JSON path in the child resource that
// holds the parent ID (e.g. "spec.clusterID"). urlParam is the chi URL parameter name
// containing the actual parent ID value from the request path.
func parentFilterMiddleware(idField, urlParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parentID := chi.URLParam(r, urlParam)
			ctx := handlers.WithParentFilter(r.Context(), handlers.ParentFilter{
				IDField: idField,
				ID:      parentID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// setupConvertingRouter configures the HTTP router with converting handlers for public API.
// publicRegistry defines the public API types and schemas.
// privateRegistry provides the shared storage backend.
func setupConvertingRouter(publicRegistry *ResourceRegistry, privateRegistry *ResourceRegistry, converter *conversion.Converter, privateScheme *runtime.Scheme, corsOrigins []string, customMiddleware []func(http.Handler) http.Handler) (chi.Router, error) {
	r := chi.NewRouter()

	// Add CORS middleware
	r.Use(middleware.CORS(middleware.CORSOptions{
		AllowedOrigins: corsOrigins,
	}))

	for _, mw := range customMiddleware {
		r.Use(mw)
	}

	// Create discovery handler using public registry (for public API types)
	discoveryHandler := handlers.NewDiscoveryHandler(publicRegistry)

	// Discovery endpoints (must be registered BEFORE resource routes to avoid shadowing)
	r.Get("/apis", discoveryHandler.APIGroupList)
	r.Get("/openapi/v2", discoveryHandler.OpenAPIV2)
	r.Get("/openapi/v3", discoveryHandler.OpenAPIV3)

	// Group resources by GroupVersion (from public registry)
	gvResources := make(map[string][]ResourceInfo)
	for _, res := range publicRegistry.GetResources() {
		gv := fmt.Sprintf("%s/%s", res.GVK.Group, res.GVK.Version)
		gvResources[gv] = append(gvResources[gv], res)
	}

	// Setup routes for each GroupVersion
	for gv, resources := range gvResources {
		group := resources[0].GVK.Group
		version := resources[0].GVK.Version
		apiPath := "/apis/" + gv

		r.Route(apiPath, func(r chi.Router) {
			// Discovery endpoint for this specific group/version (before namespaced routes)
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				discoveryHandler.APIResourceList(w, req, group, version)
			})

			// Cluster-scoped resources get CRUD directly under the GV path
			for _, res := range resources {
				if res.Namespaced {
					continue
				}
				handlerInterface, err := createConvertingHandlerWithSharedStore(publicRegistry, privateRegistry, converter, privateScheme, res)
				if err != nil {
					continue
				}
				handler := handlerInterface.(*handlers.ConvertingResourceHandler)
				plural := res.Plural
				r.Post("/"+plural, handler.Create)
				r.Get("/"+plural, handler.List)
				r.Get("/"+plural+"/{name}", handler.Get)
				r.Put("/"+plural+"/{name}", handler.Update)
				r.Patch("/"+plural+"/{name}", handler.Patch)
				r.Delete("/"+plural+"/{name}", handler.Delete)
				r.Put("/"+plural+"/{name}/status", handler.UpdateStatus)
			}

			// Namespaced resources: LIST across all namespaces + CRUD under /namespaces/{namespace}
			var namespacedHandlers []struct {
				res     ResourceInfo
				handler *handlers.ConvertingResourceHandler
			}
			for _, res := range resources {
				if !res.Namespaced {
					continue
				}
				handlerInterface, err := createConvertingHandlerWithSharedStore(publicRegistry, privateRegistry, converter, privateScheme, res)
				if err != nil {
					continue
				}
				handler := handlerInterface.(*handlers.ConvertingResourceHandler)
				r.Get("/"+res.Plural, handler.List)
				namespacedHandlers = append(namespacedHandlers, struct {
					res     ResourceInfo
					handler *handlers.ConvertingResourceHandler
				}{res, handler})
			}
			if len(namespacedHandlers) > 0 {
				r.Route("/namespaces/{namespace}", func(r chi.Router) {
					for _, nh := range namespacedHandlers {
						plural := nh.res.Plural
						r.Post("/"+plural, nh.handler.Create)
						r.Get("/"+plural, nh.handler.List)
						r.Get("/"+plural+"/{name}", nh.handler.Get)
						r.Put("/"+plural+"/{name}", nh.handler.Update)
						r.Patch("/"+plural+"/{name}", nh.handler.Patch)
						r.Delete("/"+plural+"/{name}", nh.handler.Delete)
						r.Put("/"+plural+"/{name}/status", nh.handler.UpdateStatus)

						// Nested routes under parent resource (e.g. /clusters/{clusterID}/nodepools)
						if p := nh.res.ParentResource; p != nil {
							idField := p.IDField
							r.Route("/"+p.Plural+"/{parentID}/"+plural, func(r chi.Router) {
								r.Use(parentFilterMiddleware(idField, "parentID"))
								r.Post("/", nh.handler.Create)
								r.Get("/", nh.handler.List)
								r.Get("/{name}", nh.handler.Get)
								r.Put("/{name}", nh.handler.Update)
								r.Patch("/{name}", nh.handler.Patch)
								r.Delete("/{name}", nh.handler.Delete)
								r.Put("/{name}/status", nh.handler.UpdateStatus)
							})
						}
					}
				})
			}
		})

		// Per-group discovery endpoint
		r.Get("/apis/"+group, func(w http.ResponseWriter, req *http.Request) {
			discoveryHandler.APIGroup(w, req, group)
		})

		// OpenAPI v3 per-group-version endpoint
		r.Get("/openapi/v3/apis/"+gv, func(w http.ResponseWriter, req *http.Request) {
			discoveryHandler.OpenAPIV3GroupVersion(w, req, group, version)
		})
	}

	return r, nil
}
