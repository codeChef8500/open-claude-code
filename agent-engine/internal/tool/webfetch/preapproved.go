package webfetch

import "strings"

// preapprovedHosts is the set of code-related domains that are allowed without
// user permission, matching claude-code-main's PREAPPROVED_HOSTS list.
// SECURITY: These are ONLY for WebFetch (GET requests only).
var preapprovedHosts = map[string]bool{
	// Anthropic
	"platform.claude.com":   true,
	"code.claude.com":       true,
	"modelcontextprotocol.io": true,
	"agentskills.io":        true,

	// Top Programming Languages
	"docs.python.org":        true,
	"en.cppreference.com":    true,
	"docs.oracle.com":        true,
	"learn.microsoft.com":    true,
	"developer.mozilla.org":  true,
	"go.dev":                 true,
	"pkg.go.dev":             true,
	"www.php.net":            true,
	"docs.swift.org":         true,
	"kotlinlang.org":         true,
	"ruby-doc.org":           true,
	"doc.rust-lang.org":      true,
	"www.typescriptlang.org": true,

	// Web & JavaScript Frameworks/Libraries
	"react.dev":        true,
	"angular.io":       true,
	"vuejs.org":        true,
	"nextjs.org":       true,
	"expressjs.com":    true,
	"nodejs.org":       true,
	"bun.sh":           true,
	"jquery.com":       true,
	"getbootstrap.com": true,
	"tailwindcss.com":  true,
	"d3js.org":         true,
	"threejs.org":      true,
	"redux.js.org":     true,
	"webpack.js.org":   true,
	"jestjs.io":        true,
	"reactrouter.com":  true,

	// Python Frameworks & Libraries
	"docs.djangoproject.com":   true,
	"flask.palletsprojects.com": true,
	"fastapi.tiangolo.com":     true,
	"pandas.pydata.org":        true,
	"numpy.org":                true,
	"www.tensorflow.org":       true,
	"pytorch.org":              true,
	"scikit-learn.org":         true,
	"matplotlib.org":           true,
	"requests.readthedocs.io":  true,
	"jupyter.org":              true,

	// PHP Frameworks
	"laravel.com":   true,
	"symfony.com":   true,
	"wordpress.org": true,

	// Java Frameworks & Libraries
	"docs.spring.io":    true,
	"hibernate.org":     true,
	"tomcat.apache.org": true,
	"gradle.org":        true,
	"maven.apache.org":  true,

	// .NET & C# Frameworks
	"asp.net":                  true,
	"dotnet.microsoft.com":     true,
	"nuget.org":                true,
	"blazor.net":               true,

	// Mobile Development
	"reactnative.dev":       true,
	"docs.flutter.dev":      true,
	"developer.apple.com":   true,
	"developer.android.com": true,

	// Data Science & Machine Learning
	"keras.io":          true,
	"spark.apache.org":  true,
	"huggingface.co":    true,
	"www.kaggle.com":    true,

	// Databases
	"www.mongodb.com":    true,
	"redis.io":           true,
	"www.postgresql.org": true,
	"dev.mysql.com":      true,
	"www.sqlite.org":     true,
	"graphql.org":        true,
	"prisma.io":          true,

	// Cloud & DevOps
	"docs.aws.amazon.com":  true,
	"cloud.google.com":     true,
	"kubernetes.io":        true,
	"www.docker.com":       true,
	"www.terraform.io":     true,
	"www.ansible.com":      true,
	"vercel.com":           true,
	"docs.netlify.com":     true,
	"devcenter.heroku.com": true,

	// Testing & Monitoring
	"cypress.io":   true,
	"selenium.dev": true,

	// Game Development
	"docs.unity.com":         true,
	"docs.unrealengine.com":  true,

	// Other Essential Tools
	"git-scm.com":      true,
	"nginx.org":        true,
	"httpd.apache.org": true,
}

// pathPrefixHosts are entries that require matching on hostname + path prefix.
// e.g. "github.com/anthropics" matches github.com/anthropics/... but not github.com/anthropics-evil.
var pathPrefixHosts = map[string][]string{
	"github.com": {"/anthropics"},
}

// IsPreapprovedHost checks whether the given hostname (and optionally path)
// is in the preapproved list for WebFetch.
func IsPreapprovedHost(hostname, pathname string) bool {
	if preapprovedHosts[hostname] {
		return true
	}
	// Check path-scoped entries.
	prefixes, ok := pathPrefixHosts[hostname]
	if !ok {
		return false
	}
	for _, p := range prefixes {
		if pathname == p || strings.HasPrefix(pathname, p+"/") {
			return true
		}
	}
	return false
}

// IsPreapprovedURL parses a URL and checks if it belongs to a preapproved host.
func IsPreapprovedURL(rawURL string) bool {
	// Quick hostname extraction without full url.Parse.
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return false
	}
	rest := rawURL[idx+3:]
	slashIdx := strings.Index(rest, "/")
	hostname := rest
	pathname := "/"
	if slashIdx >= 0 {
		hostname = rest[:slashIdx]
		pathname = rest[slashIdx:]
	}
	// Strip port.
	if colonIdx := strings.LastIndex(hostname, ":"); colonIdx >= 0 {
		hostname = hostname[:colonIdx]
	}
	return IsPreapprovedHost(hostname, pathname)
}
