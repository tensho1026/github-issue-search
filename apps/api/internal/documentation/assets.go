package apidocs

const indexHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="robots" content="noindex,nofollow">
    <title>IssueScout API reference</title>
    <link rel="icon" type="image/png" sizes="32x32" href="/docs/favicon-32x32.png">
    <link rel="stylesheet" href="/docs/swagger-ui.css">
    <link rel="stylesheet" href="/docs/issuescout-swagger.css">
  </head>
  <body>
    <a class="skip-link" href="#swagger-ui">Skip to API operations</a>
    <header class="docs-header">
      <div>
        <p class="docs-eyebrow">IssueScout</p>
        <h1>API reference</h1>
        <p>
          Explore the versioned contract, inspect realistic schemas, and send
          requests to this API process.
        </p>
      </div>
      <nav aria-label="API reference resources">
        <a href="/openapi.yaml" download="issuescout-openapi.yaml">Download OpenAPI YAML</a>
        <a href="https://github.com/tensho1026/github-issue-search">Source repository</a>
      </nav>
    </header>
    <main id="swagger-ui" aria-label="Interactive API operations" aria-busy="true"></main>
    <noscript>
      <p class="docs-noscript">
        JavaScript is required for the interactive reference. The
        <a href="/openapi.yaml">OpenAPI YAML remains available</a>.
      </p>
    </noscript>
    <script src="/docs/swagger-ui-bundle.js"></script>
    <script src="/docs/issuescout-swagger.js"></script>
  </body>
</html>
`

const bootstrapJavaScript = `(() => {
  "use strict";

  const initialize = () => {
    const root = document.querySelector("#swagger-ui");
    if (typeof window.SwaggerUIBundle !== "function") {
      root?.setAttribute("aria-busy", "false");
      if (root) {
        root.textContent = "The embedded API reference could not be loaded.";
      }
      return;
    }

    window.ui = window.SwaggerUIBundle({
      url: "/openapi.yaml",
      dom_id: "#swagger-ui",
      deepLinking: true,
      defaultModelExpandDepth: 2,
      defaultModelsExpandDepth: 1,
      displayOperationId: true,
      displayRequestDuration: true,
      docExpansion: "list",
      filter: true,
      persistAuthorization: false,
      queryConfigEnabled: false,
      requestSnippetsEnabled: true,
      showCommonExtensions: true,
      showExtensions: true,
      supportedSubmitMethods: ["get", "post", "put", "patch", "delete"],
      tryItOutEnabled: true,
      validatorUrl: null,
      withCredentials: true,
      requestInterceptor: (request) => {
        request.headers = request.headers || {};
        if (!request.headers["X-Request-ID"]) {
          const suffix =
            typeof window.crypto?.randomUUID === "function"
              ? window.crypto.randomUUID()
              : Date.now().toString(36);
          request.headers["X-Request-ID"] = "docs_" + suffix;
        }
        return request;
      },
      onComplete: () => {
        root?.setAttribute("aria-busy", "false");
      },
      presets: [window.SwaggerUIBundle.presets.apis],
      layout: "BaseLayout",
    });
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initialize, { once: true });
  } else {
    initialize();
  }
})();
`

const documentationCSS = `:root {
  color-scheme: light;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI",
    sans-serif;
}

html {
  scroll-padding-top: 1rem;
}

body {
  margin: 0;
  background: #f7f8fb;
  color: #172033;
}

.skip-link {
  position: fixed;
  z-index: 100;
  top: 0.75rem;
  left: 0.75rem;
  padding: 0.65rem 0.9rem;
  border-radius: 0.5rem;
  background: #111827;
  color: #fff;
  transform: translateY(-180%);
}

.skip-link:focus {
  transform: translateY(0);
}

.docs-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 2rem;
  padding: 2rem clamp(1rem, 4vw, 4rem);
  border-bottom: 1px solid #d7dce5;
  background: #fff;
}

.docs-header h1,
.docs-header p {
  margin: 0;
}

.docs-header h1 {
  margin-top: 0.25rem;
  font-size: clamp(1.8rem, 4vw, 2.7rem);
}

.docs-header h1 + p {
  max-width: 48rem;
  margin-top: 0.65rem;
  color: #45516a;
  line-height: 1.55;
}

.docs-eyebrow {
  color: #1769aa;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.docs-header nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.docs-header nav a {
  display: inline-flex;
  min-height: 2.75rem;
  align-items: center;
  padding: 0 1rem;
  border: 1px solid #1769aa;
  border-radius: 0.55rem;
  color: #0c558f;
  font-weight: 650;
  text-decoration: none;
}

.docs-header nav a:hover,
.docs-header nav a:focus-visible {
  background: #e8f2fb;
  outline: 3px solid #9bc7ea;
  outline-offset: 2px;
}

#swagger-ui {
  min-height: 70vh;
}

.docs-noscript {
  margin: 2rem;
  padding: 1rem;
  border: 1px solid #b42318;
  border-radius: 0.5rem;
  background: #fff4f2;
}

.swagger-ui .info {
  margin: 2rem 0;
}

.swagger-ui .scheme-container {
  box-shadow: none;
  border-block: 1px solid #d7dce5;
}

@media (max-width: 760px) {
  .docs-header {
    display: grid;
    gap: 1.25rem;
  }

  .docs-header nav {
    display: grid;
  }

  .docs-header nav a {
    justify-content: center;
  }

  .swagger-ui .wrapper {
    padding-inline: 0.65rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
  }
}
`
