package web

import "github.com/found-cake/cyber-dashboard/internal/web/httpapi"

type Dependencies = httpapi.Dependencies
type Server = httpapi.Server
type ArticleEnricher = httpapi.ArticleEnricher
type VulnerabilityEnricher = httpapi.VulnerabilityEnricher

func NewServer(dependencies Dependencies) *Server {
	return httpapi.NewServer(dependencies)
}
