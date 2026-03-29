package middleware_test

import (\n    "net/http"\n    "net/http/httptest"\n    "testing"\n)

// Test exampleMiddleware to check if it works as intended.
func TestExampleMiddleware(t *testing.T) {\n    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n        w.WriteHeader(http.StatusOK)\n    })\n
    // Create a request to send to the handler.
    req, err := http.NewRequest("GET", "/", nil)\n    if err != nil {\n        t.Fatal(err)\n    }

    // Create a ResponseRecorder to record the response.
    rr := httptest.NewRecorder()\n    mw := ExampleMiddleware(handler)\n
    // Perform the request using the middleware.
    mw.ServeHTTP(rr, req)

    // Check the status code is what we expect.
    if status := rr.Code; status != http.StatusOK {\n        t.Errorf("handler returned wrong status code: expected %v, got %v", http.StatusOK, status)\n    }\n}