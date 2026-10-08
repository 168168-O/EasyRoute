(() => {
  if (window.__dahuoWatch) return;
  window.__dahuoWatch = true;
  const seen = new Set();
  function textOf(node) {
    if (!node || node.nodeType !== 1) return "";
    const tag = (node.tagName || "").toLowerCase();
    if (tag === "script" || tag === "style" || tag === "textarea" || tag === "input") return "";
    const raw = (node.innerText || "").replace(/\s+/g, " ").trim();
    if (!raw || raw.length > 400) return "";
    return raw;
  }
  function whoOf(node) {
    const r = node.getBoundingClientRect();
    if (!r || r.width < 8 || r.height < 8) return "";
    const mid = (r.left + r.width / 2);
    return mid < window.innerWidth * 0.62 ? "them" : "me";
  }
  function consider(node) {
    const text = textOf(node);
    if (!text) return;
    const who = whoOf(node);
    const key = who + "|" + text;
    if (seen.has(key)) return;
    seen.add(key);
    if (who === "them") {
      chrome.webview.postMessage(JSON.stringify({ type: "message", who: "them", text: text }));
    }
  }
  function markExisting() {
    const nodes = document.querySelectorAll("div, p, span, li");
    for (let i = 0; i < nodes.length; i++) {
      const text = textOf(nodes[i]);
      if (!text) continue;
      seen.add(whoOf(nodes[i]) + "|" + text);
    }
  }
  function unread() {
    let count = 0;
    const nodes = document.querySelectorAll("span, em, i, sup");
    for (let i = 0; i < nodes.length; i++) {
      const t = (nodes[i].textContent || "").trim();
      if (!/^\d{1,3}$/.test(t)) continue;
      const r = nodes[i].getBoundingClientRect();
      if (r.width > 0 && r.width < 36 && r.height < 36) count += parseInt(t, 10);
    }
    chrome.webview.postMessage(JSON.stringify({ type: "unread", count: count }));
  }
  markExisting();
  const obs = new MutationObserver((records) => {
    for (let i = 0; i < records.length; i++) {
      const added = records[i].addedNodes;
      for (let j = 0; j < added.length; j++) consider(added[j]);
    }
  });
  obs.observe(document.documentElement, { childList: true, subtree: true });
  setInterval(unread, 20000);
})();
