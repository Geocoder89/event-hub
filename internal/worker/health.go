package worker

import "net/http"



func HealthHandler() http.Handler {
	mux := http.NewServeMux()


	mux.HandleFunc("/healthz", func(w http.ResponseWriter,_ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return mux
}