"use strict";

(() => {
  const API_BASE = "/api/v1";
  let bearerToken = null;

  const $ = (id) => document.getElementById(id);
  const signInSection = $("sign-in");
  const workspace = $("workspace");
  const globalMessage = $("global-message");
  const signInForm = $("sign-in-form");
  const directoryForm = $("directory-form");
  const directoryResults = $("directory-results");
  const auditResults = $("audit-results");

  function showMessage(element, message, isError = false) {
    element.textContent = message;
    element.classList.toggle("error", isError);
    element.hidden = !message;
  }

  function clearMessage(element) {
    showMessage(element, "");
  }

  async function request(path, options = {}) {
    const headers = new Headers(options.headers || {});
    headers.set("Accept", "application/json");
    if (options.body !== undefined) headers.set("Content-Type", "application/json");
    if (options.auth !== false && bearerToken) {
      headers.set("Authorization", `Bearer ${bearerToken}`);
    }

    const response = await fetch(`${API_BASE}${path}`, {
      method: options.method || "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      credentials: "omit",
      cache: "no-store",
      redirect: "error"
    });
    const text = await response.text();
    let data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch {
        data = text;
      }
    }
    if (!response.ok) {
      const detail = data && typeof data === "object"
        ? (data.error?.message || data.message || data.error || data.detail)
        : data;
      throw new Error(`Erro HTTP ${response.status}${detail ? `: ${String(detail).slice(0, 300)}` : ""}`);
    }
    return data;
  }

  function getSessionData(data) {
    if (!data || typeof data !== "object") return {};
    return data.session && typeof data.session === "object" ? { ...data, ...data.session } : data;
  }

  function getToken(data) {
    const session = getSessionData(data);
    const token = session.access_token || session.token || session.bearer_token;
    return typeof token === "string" && token.trim() ? token.trim() : null;
  }

  function readable(value) {
    if (value === null || value === undefined || value === "") return "—";
    if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
      return String(value);
    }
    try {
      return JSON.stringify(value);
    } catch {
      return "[valor indisponível]";
    }
  }

  function displayIdentity(data) {
    const session = getSessionData(data);
    const principal = session.principal && typeof session.principal === "object"
      ? session.principal
      : session.user && typeof session.user === "object" ? session.user : session;
    const account = principal.display_name || principal.account || principal.username || principal.name ||
      principal.account_id || session.account || session.username || session.account_id;
    const roles = principal.role ?? principal.roles ?? session.role ?? session.roles;
    $("identity-account").textContent = readable(account);
    $("identity-role").textContent = roles === undefined ? "Não informada" : readable(roles);
  }

  function normalizeRecords(data) {
    if (Array.isArray(data)) return data;
    if (!data || typeof data !== "object") return [];
    if (Array.isArray(data.accounts) || Array.isArray(data.characters)) {
      return [
        ...(data.accounts || []).map((record) => ({ ...record, record_type: "Conta" })),
        ...(data.characters || []).map((record) => ({ ...record, record_type: "Personagem" }))
      ];
    }
    for (const key of ["results", "items", "records", "accounts", "characters", "entries", "events", "audit"]) {
      if (Array.isArray(data[key])) return data[key];
    }
    if (Array.isArray(data.data)) return data.data;
    return [];
  }

  function recordTitle(record, fallback) {
    if (!record || typeof record !== "object") return fallback;
    const title = record.display_name || record.name || record.username || record.account_name ||
      record.character_name || record.id || record.account_id || record.character_id;
    return title === undefined ? fallback : readable(title);
  }

  function renderRecords(container, data, emptyText, label) {
    container.replaceChildren();
    const records = normalizeRecords(data);
    if (!records.length) {
      const empty = document.createElement("p");
      empty.className = "empty-state";
      empty.textContent = emptyText;
      container.append(empty);
      return;
    }

    for (const [index, record] of records.slice(0, 100).entries()) {
      const card = document.createElement("article");
      card.className = "result-card";
      const heading = document.createElement("h3");
      const recordType = record && typeof record === "object" ? `${record.record_type || label} · ` : "";
      heading.textContent = `${recordType}${recordTitle(record, `${label} ${index + 1}`)}`;
      card.append(heading);

      if (record && typeof record === "object" && !Array.isArray(record)) {
        const details = document.createElement("dl");
        for (const [key, value] of Object.entries(record).slice(0, 30)) {
          const term = document.createElement("dt");
          const description = document.createElement("dd");
          term.textContent = key;
          description.textContent = readable(value);
          details.append(term, description);
        }
        card.append(details);
      } else {
        const description = document.createElement("p");
        description.textContent = readable(record);
        card.append(description);
      }
      container.append(card);
    }
    if (records.length > 100) {
      const note = document.createElement("p");
      note.className = "empty-state";
      note.textContent = "Exibindo os primeiros 100 registros.";
      container.append(note);
    }
  }

  async function loadAudit() {
    const message = $("audit-message");
    const button = $("refresh-audit");
    clearMessage(message);
    button.disabled = true;
    try {
      const data = await request("/audit");
      renderRecords(auditResults, data, "Nenhum registro de auditoria retornado.", "Registro");
    } catch (error) {
      showMessage(message, error.message || "Não foi possível carregar a auditoria.", true);
    } finally {
      button.disabled = false;
    }
  }

  function enterWorkspace(data) {
    displayIdentity(data);
    signInSection.hidden = true;
    workspace.hidden = false;
    clearMessage(globalMessage);
    loadAudit();
  }

  function endSession(message) {
    bearerToken = null;
    workspace.hidden = true;
    signInSection.hidden = false;
    signInForm.reset();
    directoryForm.reset();
    directoryResults.replaceChildren();
    auditResults.replaceChildren();
    $("identity-account").textContent = "—";
    $("identity-role").textContent = "Não informada";
    showMessage(globalMessage, message);
  }

  signInForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const button = signInForm.querySelector("button[type=submit]");
    button.disabled = true;
    clearMessage(globalMessage);
    try {
      const formData = new FormData(signInForm);
      const data = await request("/session", {
        method: "POST",
        auth: false,
        body: {
          username: String(formData.get("username") || ""),
          password: String(formData.get("password") || "")
        }
      });
      const token = getToken(data);
      if (!token) throw new Error("A resposta não incluiu um token de sessão reconhecido.");
      bearerToken = token;
      enterWorkspace(data);
    } catch (error) {
      bearerToken = null;
      showMessage(globalMessage, error.message || "Não foi possível iniciar a sessão.", true);
    } finally {
      button.disabled = false;
    }
  });

  directoryForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const button = directoryForm.querySelector("button[type=submit]");
    const message = $("directory-message");
    button.disabled = true;
    clearMessage(message);
    directoryResults.replaceChildren();
    try {
      const formData = new FormData(directoryForm);
      const kind = String(formData.get("kind") || "");
      const query = String(formData.get("query") || "").trim();
      const reason = String(formData.get("reason") || "").trim();
      const data = await request("/directory/search", {
        method: "POST",
        body: { type: kind, query, reason, page: 1, page_size: 25 }
      });
      renderRecords(directoryResults, data, "Nenhum resultado encontrado.", "Resultado");
    } catch (error) {
      showMessage(message, error.message || "Não foi possível concluir a busca.", true);
    } finally {
      button.disabled = false;
    }
  });

  $("refresh-audit").addEventListener("click", loadAudit);

  $("logout-button").addEventListener("click", async (event) => {
    const button = event.currentTarget;
    button.disabled = true;
    let message = "Sessão encerrada.";
    try {
      await request("/session", { method: "DELETE" });
    } catch (error) {
      message = `Sessão removida deste painel, mas a revogação no servidor falhou: ${error.message}`;
    } finally {
      endSession(message);
      button.disabled = false;
    }
  });
})();
