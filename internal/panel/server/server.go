package server

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync/atomic"

	"mirai/internal/panel/secure"
	"mirai/internal/panel/settings"
)

// Server routes by the first path segment: the secret admin path, the subscription
// path, or the public website. Secret paths never appear on the public website.
type Server struct {
	paths      atomic.Pointer[settings.Paths]
	admin      http.Handler
	sub        http.Handler
	publicSub  func(*http.Request) string
	camouflage http.Handler
	legacy     http.Handler // the old panel's links (settings.Paths.Legacy); nil: none
	hsts       atomic.Pointer[func() bool]
}

// SetLegacy takes the handler of the old panel's subscription links.
func (s *Server) SetLegacy(h http.Handler) { s.legacy = h }

func New(admin, sub http.Handler) *Server {
	s := &Server{admin: admin, sub: sub}
	s.paths.Store(&settings.Paths{})
	return s
}

func (s *Server) SetSubscriptionURL(get func(*http.Request) string) { s.publicSub = get }
func (s *Server) SetCamouflage(h http.Handler)                      { s.camouflage = h }

func (s *Server) SetPaths(p settings.Paths) { s.paths.Store(&p) }

// SetHSTS makes the answers tell browsers to use HTTPS for this host from now on, while on
// says so. Only for a panel that serves TLS itself, and only while its certificate is one
// browsers trust: a browser that remembers HSTS offers no way past a certificate warning,
// so a panel that falls back to its self-signed certificate (a renewal that failed, a
// custom one that expired) would lock its admin out. nil: never.
func (s *Server) SetHSTS(on func() bool) {
	if on == nil {
		s.hsts.Store(nil)
		return
	}
	s.hsts.Store(&on)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.serve(w, r, true) }

// SubOnly serves the subscription path alone, for the subscription port: the admin panel
// is not there, and its path gets the same 404 as any other.
func (s *Server) SubOnly() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.serve(w, r, false) })
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, admin bool) {
	SecurityHeaders(w.Header())
	if on := s.hsts.Load(); on != nil && (*on)() {
		w.Header().Set("Strict-Transport-Security", HSTSValue)
	}
	p := r.URL.Path
	// Reject non-canonical paths ("//", "/./", "/../") instead of guessing what they mean.
	if p == "" || p[0] != '/' || (path.Clean(p) != p && path.Clean(p)+"/" != p) {
		NotFound(w)
		return
	}
	seg, rest, _ := strings.Cut(p[1:], "/")
	paths := s.paths.Load()
	switch {
	case admin && seg != "" && paths.Admin != "" && secure.Equal(seg, paths.Admin):
		s.forward(w, r, s.admin, seg, rest, p)
	case seg != "" && paths.Sub != "" && secure.Equal(seg, paths.Sub):
		s.forward(w, r, s.sub, seg, rest, p)
	case s.legacy != nil && paths.Legacy != "" && strings.HasPrefix(p, "/"+paths.Legacy+"/"):
		// The old panel's links: on every port, as the old panel had them on its own.
		r2 := r.Clone(r.Context())
		r2.URL.Path = p[len(paths.Legacy)+1:]
		r2.URL.RawPath = ""
		s.legacy.ServeHTTP(w, r2)
	default:
		if s.camouflage != nil && (p == "/" || p == "/robots.txt" || p == "/favicon.ico") {
			s.camouflage.ServeHTTP(w, r)
			return
		}
		if s.publicSub != nil {
			if u, err := url.Parse(s.publicSub(r)); err == nil && u.Host != "" && strings.EqualFold(strings.Split(r.Host, ":")[0], u.Hostname()) {
				prefix := strings.TrimRight(u.Path, "/")
				if strings.HasPrefix(p, prefix+"/") {
					r2 := r.Clone(context.WithValue(r.Context(), pagePrefixKey{}, strings.Trim(prefix, "/")))
					r2.URL.Path = strings.TrimPrefix(p, prefix)
					r2.URL.RawPath = ""
					if s.camouflage == nil {
						s.sub.ServeHTTP(w, r2)
					} else {
						response := &subscriptionResponse{ResponseWriter: w}
						s.sub.ServeHTTP(response, r2)
						if response.missing {
							w.Header().Del("Content-Length")
							s.camouflage.ServeHTTP(w, r)
						}
					}
					return
				}
			}
		}
		if s.camouflage != nil {
			s.camouflage.ServeHTTP(w, r)
		} else {
			NotFound(w)
		}
	}
}

func (s *Server) forward(w http.ResponseWriter, r *http.Request, h http.Handler, seg, rest, full string) {
	if !strings.HasPrefix(full[1+len(seg):], "/") {
		// "/<secret>" without the trailing slash: relative asset URLs need the slash.
		http.Redirect(w, r, "/"+seg+"/", http.StatusFound)
		return
	}
	r2 := r.Clone(context.WithValue(r.Context(), pagePrefixKey{}, seg))
	r2.URL.Path = "/" + rest
	r2.URL.RawPath = ""
	h.ServeHTTP(w, r2)
}

// HSTSValue: a year, for this host only. includeSubDomains and preload are left out: the
// panel does not know what else the host's domain serves.
const HSTSValue = "max-age=31536000"

func SecurityHeaders(h http.Header) {
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
}

func NotFound(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("404 Not Found\n"))
}

type pagePrefixKey struct{}

// Keep subscription responses streaming; only discard an unknown subscription's 404.
type subscriptionResponse struct {
	http.ResponseWriter
	missing bool
}

func (w *subscriptionResponse) WriteHeader(status int) {
	w.missing = status == http.StatusNotFound
	if !w.missing {
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *subscriptionResponse) Write(body []byte) (int, error) {
	if w.missing {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}
