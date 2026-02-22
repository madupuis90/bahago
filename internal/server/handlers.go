package server

import (
	"net/http"
	"strconv"

	db "bahago/internal/database/sqlc"
	"bahago/internal/ui/pages"
)

func (s *Server) handleHomePage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		queries := db.New(s.db)
		resources, err := queries.ListResources(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		p := pages.Home(resources)
		p.Render(w)
	}
}

func (s *Server) handleAboutPage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.About()
		p.Render(w)
	}
}

func (s *Server) handleLoginPage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.Login()
		p.Render(w)
	}
}

func (s *Server) handleTestPage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.Test()
		p.Render(w)
	}
}

func (s *Server) handleCreateResources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		woodStr := r.FormValue("wood")
		stoneStr := r.FormValue("stone")
		foodStr := r.FormValue("food")

		wood, err := strconv.Atoi(woodStr)
		if err != nil {
			http.Error(w, "wood must be a number", http.StatusBadRequest)
			return
		}
		stone, err := strconv.Atoi(stoneStr)
		if err != nil {
			http.Error(w, "stone must be a number", http.StatusBadRequest)
			return
		}
		food, err := strconv.Atoi(foodStr)
		if err != nil {
			http.Error(w, "food must be a number", http.StatusBadRequest)
			return
		}

		queries := db.New(s.db)
		_, err = queries.CreateResource(r.Context(), db.CreateResourceParams{
			Wood:  int32(wood),
			Stone: int32(stone),
			Food:  int32(food),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 5. redirect back to home
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
