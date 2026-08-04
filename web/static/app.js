const form = document.getElementById("shorten-form");
const urlInput = document.getElementById("url");
const ttlSelect = document.getElementById("ttl");
const submitButton = document.getElementById("submit");
const errorBox = document.getElementById("error");
const result = document.getElementById("result");
const shortLink = document.getElementById("short-url");
const longUrlOut = document.getElementById("long-url");
const expiresOut = document.getElementById("expires");
const copyButton = document.getElementById("copy");

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  hideError();

  const url = urlInput.value.trim();
  if (!url) {
    showError("Enter a URL to shorten.");
    return;
  }

  const body = { url };
  // An absent ttl_seconds is what the API reads as "never expires" -- sending
  // 0 or null would not mean the same thing.
  if (ttlSelect.value) {
    body.ttl_seconds = Number(ttlSelect.value);
  }

  setBusy(true);
  try {
    showResult(await shorten(body));
  } catch (err) {
    result.hidden = true;
    showError(err.message);
  } finally {
    setBusy(false);
  }
});

copyButton.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(shortLink.textContent);
    flashCopied("Copied");
  } catch {
    // Clipboard access needs a secure context, so this is the normal path on
    // plain HTTP away from localhost. Select the link instead so Cmd/Ctrl+C works.
    selectText(shortLink);
    flashCopied("Press ⌘/Ctrl+C");
  }
});

async function shorten(body) {
  let response;
  try {
    response = await fetch("/api/v1/shorten", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    throw new Error("Could not reach the server.");
  }

  const payload = await response.json().catch(() => null);

  if (!response.ok) {
    // Errors come back as {"error": "..."}; fall back to the status if the
    // body is not JSON (a proxy error page, say).
    throw new Error(payload?.error || `Request failed (${response.status}).`);
  }
  if (!payload?.short_url) {
    throw new Error("Unexpected response from the server.");
  }
  return payload;
}

function showResult(link) {
  // short_url is built from the server's BASE_URL and can legitimately differ
  // from this page's origin, so display what came back rather than rebuilding it.
  shortLink.textContent = link.short_url;
  shortLink.href = isHttpURL(link.short_url) ? link.short_url : "";

  longUrlOut.textContent = link.long_url;
  expiresOut.textContent = link.expires_at
    ? new Date(link.expires_at).toLocaleString()
    : "Never";

  copyButton.textContent = "Copy";
  result.hidden = false;
}

function isHttpURL(value) {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

function setBusy(busy) {
  submitButton.disabled = busy;
  submitButton.textContent = busy ? "Shortening…" : "Shorten";
}

function showError(message) {
  errorBox.textContent = message;
  errorBox.hidden = false;
}

function hideError() {
  errorBox.hidden = true;
  errorBox.textContent = "";
}

let copyResetTimer;
function flashCopied(label) {
  copyButton.textContent = label;
  clearTimeout(copyResetTimer);
  copyResetTimer = setTimeout(() => {
    copyButton.textContent = "Copy";
  }, 1500);
}

function selectText(node) {
  const range = document.createRange();
  range.selectNodeContents(node);
  const selection = window.getSelection();
  selection.removeAllRanges();
  selection.addRange(range);
}
