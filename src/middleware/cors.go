package middleware

import "net/http"

// CORS sets strict CORS headers and handles OPTIONS requests.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://bincrypt.io")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, BTCPay-Sig, BTCPay-Nonce")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
