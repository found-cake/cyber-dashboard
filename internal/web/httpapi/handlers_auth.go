package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/auth"
	"github.com/labstack/echo/v5"
)

const (
	accessCookieName           = "cyber_dashboard_access"
	refreshCookieName          = "cyber_dashboard_refresh"
	maxAuthenticationBodyBytes = 4 * 1024
)

func (s *Server) requireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		authenticated, err := s.authenticated(c)
		if err != nil {
			return writeAPIError(c, err)
		}
		if authenticated {
			return next(c)
		}
		return writeAuthenticationRequired(c)
	}
}

func (s *Server) authenticated(c *echo.Context) (bool, error) {
	if s.auth == nil {
		return true, nil
	}
	cookie, err := c.Request().Cookie(accessCookieName)
	if err != nil {
		return false, nil
	}
	err = s.auth.VerifyAccess(c.Request().Context(), cookie.Value)
	if errors.Is(err, auth.ErrInvalidToken) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Server) login(c *echo.Context) error {
	if s.auth == nil {
		return c.JSON(http.StatusOK, api.AuthState{Authenticated: true})
	}
	var request api.LoginRequest
	if err := decodeAuthenticationRequest(c, &request); err != nil {
		return writeAuthenticationRequestError(c, err)
	}
	client := loginClient(c.Request())
	if !s.loginLimiter.take(client) {
		c.Response().Header().Set("Retry-After", "60")
		return c.JSON(http.StatusTooManyRequests, localizedError("login_rate_limited",
			"로그인 시도가 너무 많습니다. 1분 후 다시 시도하세요", "Too many login attempts. Try again in one minute"))
	}
	pair, err := s.auth.Login(c.Request().Context(), request.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		return c.JSON(http.StatusUnauthorized, localizedError("invalid_credentials",
			"비밀번호가 올바르지 않습니다", "The password is incorrect"))
	}
	if err != nil {
		return writeAPIError(c, err)
	}
	s.loginLimiter.reset(client)
	s.setAuthCookies(c, pair)
	return c.JSON(http.StatusOK, api.AuthState{Enabled: true, Authenticated: true})
}

func loginClient(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func (s *Server) refreshSession(c *echo.Context) error {
	if s.auth == nil {
		return c.JSON(http.StatusOK, api.AuthState{Authenticated: true})
	}
	cookie, err := c.Request().Cookie(refreshCookieName)
	if err != nil {
		return writeAuthenticationRequired(c)
	}
	pair, err := s.auth.Refresh(c.Request().Context(), cookie.Value)
	if errors.Is(err, auth.ErrInvalidToken) {
		s.clearAuthCookies(c)
		return writeAuthenticationRequired(c)
	}
	if err != nil {
		return writeAPIError(c, err)
	}
	s.setAuthCookies(c, pair)
	return c.JSON(http.StatusOK, api.AuthState{Enabled: true, Authenticated: true})
}

func (s *Server) logout(c *echo.Context) error {
	if s.auth != nil {
		cookie, err := c.Request().Cookie(refreshCookieName)
		if err == nil {
			if err := s.auth.Logout(c.Request().Context(), cookie.Value); err != nil {
				s.clearAuthCookies(c)
				return writeAPIError(c, err)
			}
		}
	}
	s.clearAuthCookies(c)
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) changePassword(c *echo.Context) error {
	if s.auth == nil {
		return c.JSON(http.StatusNotFound, localizedError("authentication_disabled",
			"인증 기능이 비활성화되어 있습니다", "Authentication is disabled"))
	}
	var request api.ChangePasswordRequest
	if err := decodeAuthenticationRequest(c, &request); err != nil {
		return writeAuthenticationRequestError(c, err)
	}
	pair, err := s.auth.ChangePassword(c.Request().Context(), request.CurrentPassword, request.NewPassword)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		return c.JSON(http.StatusUnauthorized, localizedError("invalid_credentials",
			"현재 비밀번호가 올바르지 않습니다", "The current password is incorrect"))
	}
	if errors.Is(err, auth.ErrWeakPassword) {
		return c.JSON(http.StatusBadRequest, localizedError("weak_password",
			"새 비밀번호는 12바이트 이상 128바이트 이하여야 합니다", "The new password must be between 12 and 128 bytes"))
	}
	if err != nil {
		return writeAPIError(c, err)
	}
	s.setAuthCookies(c, pair)
	return c.JSON(http.StatusOK, api.AuthState{Enabled: true, Authenticated: true})
}

func decodeAuthenticationRequest(c *echo.Context, request any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAuthenticationBodyBytes)
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(request); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeAuthenticationRequestError(c *echo.Context, err error) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return c.JSON(http.StatusRequestEntityTooLarge, localizedError("request_too_large",
			"요청 본문이 너무 큽니다", "The request body is too large"))
	}
	return writeBadRequest(c, "invalid JSON body")
}

func (s *Server) setAuthCookies(c *echo.Context, pair auth.TokenPair) {
	secure := c.Request().TLS != nil
	http.SetCookie(c.Response(), &http.Cookie{Name: accessCookieName, Value: pair.AccessToken, Path: "/",
		MaxAge: int(auth.AccessLifetime.Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
	http.SetCookie(c.Response(), &http.Cookie{Name: refreshCookieName, Value: pair.RefreshToken, Path: "/api/auth",
		MaxAge: int(auth.RefreshLifetime.Seconds()), HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func (s *Server) clearAuthCookies(c *echo.Context) {
	secure := c.Request().TLS != nil
	for _, cookie := range []*http.Cookie{
		{Name: accessCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode},
		{Name: refreshCookieName, Path: "/api/auth", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode},
	} {
		http.SetCookie(c.Response(), cookie)
	}
}

func writeAuthenticationRequired(c *echo.Context) error {
	return c.JSON(http.StatusUnauthorized, localizedError("authentication_required",
		"로그인이 필요합니다", "Authentication is required"))
}
