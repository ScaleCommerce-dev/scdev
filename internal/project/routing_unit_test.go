package project

import (
	"testing"

	"github.com/0ploy/zdev/internal/config"
	"github.com/0ploy/zdev/internal/runtime"
)

func TestConfigureRouting_DefaultDomain(t *testing.T) {
	p := &Project{
		Config: &config.ProjectConfig{
			Name:   "myproject",
			Domain: "myproject.0ploy.dev",
		},
	}

	cfg := &runtime.ContainerConfig{
		Labels: make(map[string]string),
	}

	routing := &config.RoutingConfig{
		Protocol: "http",
		Port:     3000,
	}

	p.configureRouting(cfg, "app", routing, false)

	rule := cfg.Labels["traefik.http.routers.myproject-app.rule"]
	expected := "Host(`myproject.0ploy.dev`)"
	if rule != expected {
		t.Errorf("expected rule %q, got %q", expected, rule)
	}
}

func TestConfigureRouting_CustomDomain(t *testing.T) {
	p := &Project{
		Config: &config.ProjectConfig{
			Name:   "myproject",
			Domain: "myproject.0ploy.dev",
		},
	}

	cfg := &runtime.ContainerConfig{
		Labels: make(map[string]string),
	}

	routing := &config.RoutingConfig{
		Protocol: "http",
		Port:     4000,
		Domain:   "api.myproject.0ploy.dev",
	}

	p.configureRouting(cfg, "backend", routing, false)

	rule := cfg.Labels["traefik.http.routers.myproject-backend.rule"]
	expected := "Host(`api.myproject.0ploy.dev`)"
	if rule != expected {
		t.Errorf("expected rule %q, got %q", expected, rule)
	}
}

func TestConfigureRouting_CustomDomainWithTLS(t *testing.T) {
	p := &Project{
		Config: &config.ProjectConfig{
			Name:   "myproject",
			Domain: "myproject.0ploy.dev",
		},
	}

	cfg := &runtime.ContainerConfig{
		Labels: make(map[string]string),
	}

	routing := &config.RoutingConfig{
		Protocol: "http",
		Port:     4000,
		Domain:   "api.myproject.0ploy.dev",
	}

	p.configureRouting(cfg, "backend", routing, true)

	// HTTP router should use custom domain
	httpRule := cfg.Labels["traefik.http.routers.myproject-backend.rule"]
	expected := "Host(`api.myproject.0ploy.dev`)"
	if httpRule != expected {
		t.Errorf("HTTP rule: expected %q, got %q", expected, httpRule)
	}

	// HTTPS router should also use custom domain
	httpsRule := cfg.Labels["traefik.http.routers.myproject-backend-https.rule"]
	if httpsRule != expected {
		t.Errorf("HTTPS rule: expected %q, got %q", expected, httpsRule)
	}
}

func TestConfigureRouting_TCPIgnoresCustomDomain(t *testing.T) {
	p := &Project{
		Config: &config.ProjectConfig{
			Name:   "myproject",
			Domain: "myproject.0ploy.dev",
		},
	}

	cfg := &runtime.ContainerConfig{
		Labels: make(map[string]string),
	}

	routing := &config.RoutingConfig{
		Protocol: "tcp",
		Port:     5432,
		HostPort: 5432,
		Domain:   "should-be-ignored.0ploy.dev",
	}

	p.configureRouting(cfg, "db", routing, false)

	// TCP uses HostSNI(*), not Host() - domain field is irrelevant
	rule := cfg.Labels["traefik.tcp.routers.myproject-db.rule"]
	expected := "HostSNI(`*`)"
	if rule != expected {
		t.Errorf("expected TCP rule %q, got %q", expected, rule)
	}
}
