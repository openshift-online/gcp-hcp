/*
 * Sign-up SPA.
 *
 * Flow: Google Identity Services sign-in yields an ID token, which is sent with
 * the project ID on submit. The backend validates the token server-side and
 * enforces the Red Hat hosted-domain policy. Nothing here is trusted by the
 * backend: the hd hint, the displayed email, and the project ID syntax check
 * are conveniences.
 */
(() => {
  "use strict";

  const PROJECT_ID_PATTERN = /^[a-z][a-z0-9-]{4,28}[a-z0-9]$/;

  const ERROR_MESSAGES = {
    invalid_request: "The request was not valid. Check the project ID and try again.",
    unauthorized:
      "Sign-in could not be verified. This sign-up accepts only Red Hat Google Workspace accounts.",
    already_claimed:
      "This project is already administered by another user. No changes were made.",
    reserved_name: "That project ID is reserved by the platform and cannot be used here.",
    transient:
      "The service had a temporary problem. Please try again in a few minutes.",
    unavailable: "Sign-up is temporarily unavailable. Please contact the platform team.",
    not_implemented:
      "This experiment's identity verification is not wired up yet.",
  };

  // Configuration comes from the meta tag (deployed) or, for local
  // development, from window.SIGNUP_CONFIG set in the console.
  function readConfig() {
    const meta = document.querySelector('meta[name="signup-config"]');
    if (meta && meta.dataset.clientId) {
      return {
        clientId: meta.dataset.clientId,
        apiBase: meta.dataset.apiBase || "",
        hostedDomainHint: meta.dataset.hostedDomainHint || "",
      };
    }
    return window.SIGNUP_CONFIG || {};
  }

  const cfg = readConfig();
  const statusEl = document.getElementById("status");
  const resultEl = document.getElementById("result");
  const resultTitleEl = document.getElementById("result-title");
  const resultDetailEl = document.getElementById("result-detail");
  const resultStepsEl = document.getElementById("result-steps");
  const resultNextLabelEl = document.getElementById("result-next-label");
  const resultExampleEl = document.getElementById("result-example");
  const resultExampleCodeEl = document.getElementById("result-example-code");
  const resultLinksEl = document.getElementById("result-links");
  const form = document.getElementById("signup-form");
  const emailEl = document.getElementById("signed-in-email");
  const switchAccountButton = document.getElementById("switch-account");
  const projectInput = document.getElementById("project-id");
  const submitButton = document.getElementById("submit-button");
  const gsiButton = document.getElementById("gsi-button");

  /** @type {string|null} */
  let idToken = null;

  function setStatus(message, kind = "") {
    statusEl.textContent = message;
    statusEl.className = "status" + (kind ? " " + kind : "");
  }

  function fail(message) {
    setStatus(message, "error");
  }

  function clearResult() {
    resultEl.hidden = true;
    resultTitleEl.textContent = "";
    resultDetailEl.textContent = "";
    resultStepsEl.replaceChildren();
  }

  /**
   * Renders a durable outcome with follow-up guidance. Text only (no HTML
   * injection): project IDs and emails end up here.
   */
  function showResult({ kind, title, detail, steps = [], example = "", links = [] }) {
    setStatus("");
    resultEl.hidden = false;
    resultEl.className = "result " + kind;
    resultTitleEl.textContent = title;
    resultDetailEl.textContent = detail;
    resultStepsEl.replaceChildren(
      ...steps.map((step) => {
        const li = document.createElement("li");
        li.textContent = step;
        return li;
      })
    );
    resultExampleEl.hidden = !example;
    resultExampleCodeEl.textContent = example;
    resultLinksEl.replaceChildren(
      ...links.map(({ text, href }, i) => {
        const a = document.createElement("a");
        a.textContent = text;
        a.href = href;
        a.target = "_blank";
        a.rel = "noopener noreferrer";
        if (i > 0) resultLinksEl.append(document.createTextNode(" · "));
        return a;
      })
    );
    resultNextLabelEl.hidden = steps.length === 0 && !example;
  }

  function showSuccess(namespace, alreadyActive) {
    showResult({
      kind: "success",
      title: alreadyActive ? "Already activated" : "Namespace activated",
      detail: alreadyActive
        ? `${namespace} already had service-admin access for your account. Nothing was changed.`
        : `${namespace} now has you as its service-admin.`,
      steps: [
        "You can close this window — no further action is needed here.",
        `You are now the service-admin of the ${namespace} namespace, so you can grant access to other people.`,
        "Use the gcp-hcp-ctl CLI for that. The command below is an example: pick any binding name and the user's email. cluster-admin is the broadest platform role, so grant it deliberately.",
        "Keep the project ID handy: it identifies the namespace in every command.",
      ],
      example: `gcphcpctl --project ${namespace} rolebinding create alice-cluster-admin \
  --subject alice@redhat.com --platform-role cluster-admin`,
      links: [
        { text: "gcp-hcp-ctl on GitHub", href: "https://github.com/openshift-online/gcp-hcp-ctl" },
        { text: "access-management commands", href: "https://github.com/openshift-online/gcp-hcp-ctl#access-management" },
      ],
    });
  }

  // Guidance per failure category. Each entry must be actionable: what
  // happened, what the user can do, and when to stop and ask for help.
  const ERROR_GUIDANCE = {
    invalid_request: {
      title: "That project ID is not valid",
      detail: "A GCP project ID is 6–30 characters: lowercase letters, digits and hyphens, starting with a letter.",
      steps: ["Check the project ID (not the project name or number) and submit again."],
    },
    unauthorized: {
      title: "Sign-in could not be verified",
      detail: "This sign-up accepts only Red Hat Google Workspace accounts, and the sign-in must still be valid.",
      steps: [
        "Sign in again, choosing your @redhat.com account.",
        "If you are already signed in with the right account, reload the page to refresh the token.",
        "If it still fails, contact the platform team with the time of the attempt.",
      ],
    },
    already_claimed: {
      title: "This project is already administered",
      detail: "That namespace already has a service-admin, and it is a different account. Existing access was not changed.",
      steps: [
        "If you expected to be the administrator, contact the platform team — the existing binding must be removed before another account can claim it.",
        "Otherwise, ask the current administrator to grant you access through the platform API.",
        "You can close this window; nothing was modified.",
      ],
    },
    reserved_name: {
      title: "That project ID is reserved",
      detail: "The platform reserves this name for its own namespaces, so it cannot be activated here.",
      steps: ["Check that you entered the right project ID.", "If you meant a different project, submit that project ID instead."],
    },
    transient: {
      title: "Temporary failure",
      detail: "The platform could not complete the activation. No binding was created, so retrying is safe.",
      steps: [
        "Wait a few minutes and submit again.",
        "If it keeps failing, contact the platform team with the project ID and the time of the attempt.",
      ],
    },
    unavailable: {
      title: "Sign-up is unavailable",
      detail: "The service is misconfigured or its permission to activate projects is missing. Retrying will not help.",
      steps: [
        "Contact the platform team and mention that the sign-up backend cannot invoke activate-project.",
        "You can close this window; nothing was modified.",
      ],
    },
  };

  /** Decode the ID token payload for display only; the backend re-validates. */
  function decodePayload(jwt) {
    try {
      const part = jwt.split(".")[1];
      const base64 = part.replace(/-/g, "+").replace(/_/g, "/");
      const json = decodeURIComponent(
        atob(base64)
          .split("")
          .map((c) => "%" + c.charCodeAt(0).toString(16).padStart(2, "0"))
          .join("")
      );
      return JSON.parse(json);
    } catch {
      return {};
    }
  }

  function onCredential(response) {
    idToken = response.credential || null;
    if (!idToken) {
      fail("Sign-in did not return a credential. Please try again.");
      return;
    }
    const claims = decodePayload(idToken);
    emailEl.textContent = claims.email || "your account";
    // The account is chosen: the sign-in button has done its job.
    gsiButton.hidden = true;
    switchAccountButton.hidden = false;
    form.hidden = false;
    setStatus("");
  }

  // The GIS iframe paints in stages: the iframe's load event fires while the
  // "G" logo is still showing, and the label/chrome arrive from a later async
  // paint inside the frame. A short settle after load, plus a fade, means the
  // first visible state is the finished button. The longer safety timeout
  // keeps a blocked or never-firing load from leaving the page buttonless.
  function revealButtonWhenPainted() {
    const reveal = () => gsiButton.classList.add("is-ready");
    const frame = gsiButton.querySelector("iframe");
    if (frame) {
      frame.addEventListener("load", () => window.setTimeout(reveal, 700), { once: true });
    }
    window.setTimeout(reveal, 3000);
  }

  function switchAccount() {
    idToken = null;
    emailEl.textContent = "…";
    form.hidden = true;
    form.reset();
    switchAccountButton.hidden = true;
    gsiButton.hidden = false;
    clearResult();
    setStatus("");
    // Avoid silently re-selecting the same Google session.
    if (window.google && google.accounts && google.accounts.id) {
      google.accounts.id.disableAutoSelect();
    }
  }

  async function submit(event) {
    event.preventDefault();

    const projectId = projectInput.value.trim().toLowerCase();
    if (!PROJECT_ID_PATTERN.test(projectId)) {
      projectInput.setAttribute("aria-invalid", "true");
      fail("Enter a valid project ID: 6–30 characters, lowercase letters, digits, hyphens, starting with a letter.");
      return;
    }
    projectInput.removeAttribute("aria-invalid");

    clearResult();
    submitButton.disabled = true;
    setStatus("Activating the namespace…");

    try {
      const res = await fetch((cfg.apiBase || "") + "/api/v1/signup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_id: projectId,
          id_token: idToken,
        }),
      });

      const body = await res.json().catch(() => ({}));
      if (res.ok) {
        const namespace = (body && body.namespace) || projectId;
        showSuccess(namespace, body && body.status === "already_active");
        form.reset();
        return;
      }

      const code = body && body.error && body.error.code;
      const guidance = ERROR_GUIDANCE[code];
      if (guidance) {
        showResult({ kind: "error", ...guidance });
      } else {
        fail(ERROR_MESSAGES[code] || "Sign-up failed. Please try again.");
      }
    } catch {
      fail("Could not reach the sign-up service. Check your connection and try again.");
    } finally {
      submitButton.disabled = false;
    }
  }

  function init() {
    if (!cfg.clientId || cfg.clientId.indexOf("REPLACE_WITH") === 0 || cfg.clientId.indexOf("placeholder") !== -1) {
      fail("This page could not load its configuration (it may be a stale cached copy). Reload with Ctrl/Cmd+Shift+R; if it persists, clear the site data for this host.");
      return;
    }
    // An empty apiBase is valid: the API is on the same origin as the page.
    // Only a missing client ID means the configuration did not load.
    if (window.__gsiLoadFailed || !window.google || !google.accounts) {
      document.getElementById("gsi-fallback").hidden = false;
      fail("Google sign-in could not be loaded. Check your connection and reload.");
      return;
    }

    google.accounts.id.initialize({
      client_id: cfg.clientId,
      callback: onCredential,
      hd: cfg.hostedDomainHint || undefined,
    });
    google.accounts.id.renderButton(gsiButton, {
      theme: "outline",
      size: "large",
      text: "signin_with",
      width: 320,
    });
    revealButtonWhenPainted();

    window.addEventListener("gsi-failed", () => {
      document.getElementById("gsi-fallback").hidden = false;
      fail("Google sign-in could not be loaded. Check your connection and reload.");
    });

    form.addEventListener("submit", submit);
    switchAccountButton.addEventListener("click", switchAccount);
  }

  // The GIS library is loaded with `defer`; wait for the load event.
  if (document.readyState === "complete") {
    init();
  } else {
    window.addEventListener("load", init, { once: true });
  }
})();
