package web

import "github.com/found-cake/cyber-dashboard/internal/web/httpapi"

type Dependencies = httpapi.Dependencies
type Server = httpapi.Server
type ArticleEnricher = httpapi.ArticleEnricher
type VulnerabilityEnricher = httpapi.VulnerabilityEnricher

func NormalizeTrustedHost(value string) (string, error) {
	return httpapi.NormalizeTrustedHost(value)
}

func NewServer(dependencies Dependencies) *Server {
	return httpapi.NewServer(dependencies)
}
