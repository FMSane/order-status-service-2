package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Servicio que consulta al microservicio externo de autenticación.
type AuthService struct {
	authURL string
	client  *http.Client
}

type AuthUser struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	Login       string   `json:"login"`
	Enabled     bool     `json:"enabled"`
}

// Crea el servicio de autenticación tomando la URL del entorno.
func NewAuthService() *AuthService {
	return &AuthService{
		authURL: os.Getenv("AUTH_SERVICE_URL"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Verifica si el usuario tiene permiso de administrador.
func (a *AuthService) IsAdmin(user *AuthUser) bool {
	for _, perm := range user.Permissions {
		if perm == "admin" {
			return true
		}
	}
	return false
}

// Valida el token consultando a /users/current del microservicio de auth.
func (a *AuthService) ValidateToken(token string) (*AuthUser, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/users/current", a.authURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	// Hace una petición HTTP al microservicio Auth (a la ruta /users/current)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid token")
	}

	var user AuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	if !user.Enabled {
		return nil, errors.New("user disabled")
	}

	return &user, nil
}
