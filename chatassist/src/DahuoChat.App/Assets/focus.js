(() => {
  const nodes = document.querySelectorAll("textarea, [contenteditable='true']");
  for (let i = 0; i < nodes.length; i++) {
    const el = nodes[i];
    const r = el.getBoundingClientRect();
    if (r.width < 40 || r.top < window.innerHeight * 0.45) continue;
    el.focus();
    return true;
  }
  return false;
})();
