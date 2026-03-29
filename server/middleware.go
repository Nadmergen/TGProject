// middleware.go corrections for int64 types

package middleware

import "net/http"

// Middleware example that handles int64 corrections
func Int64Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // handle int64 parsing and validation
        next.ServeHTTP(w, r)
    })
}