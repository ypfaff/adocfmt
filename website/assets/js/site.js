// Search: Pagefind's modal opens with the header button and Cmd/Ctrl+K on its
// own; / and ?q=term are added here.
const modal = document.querySelector("pagefind-modal");
const trigger = document.querySelector("pagefind-modal-trigger");
document.addEventListener("keydown", (event) => {
  const typing = event.target.closest("input, textarea, [contenteditable]");
  if (event.key === "/" && !typing && !modal.isOpen) {
    event.preventDefault();
    modal.open();
  }
});
// ?q=term opens the search with that term, so a search can be linked.
const term = new URLSearchParams(location.search).get("q");
if (term) {
  customElements.whenDefined("pagefind-modal").then(() => {
    modal.open();
    const input = modal.querySelector("input");
    input.value = term;
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
// The trigger shows only its icon on narrow screens, which rarely have a keyboard.
const narrow = matchMedia("(max-width: 820px)");
const compact = () => {
  trigger?.toggleAttribute("compact", narrow.matches);
  trigger?.toggleAttribute("hide-shortcut", narrow.matches);
};
narrow.addEventListener("change", compact);
compact();

// Navigation on narrow screens.
const menuButton = document.querySelector(".menu-button");
const sidebar = document.getElementById("sidebar");
menuButton?.addEventListener("click", () => {
  const open = sidebar.classList.toggle("open");
  menuButton.setAttribute("aria-expanded", open);
});

// Before and after tables: side by side, stacked with labels on narrow screens.
document.querySelectorAll(".adoc table.tableblock").forEach((table) => {
  const headers = [...table.querySelectorAll("thead th")].map((th) => th.textContent.trim());
  if (headers.join("|") !== "Before|After") return;
  table.classList.add("before-after");
  table.querySelectorAll("tbody tr").forEach((row) => {
    [...row.children].forEach((cell, index) => (cell.dataset.label = headers[index]));
  });
});

// Copy buttons on code listings.
document.querySelectorAll(".adoc .listingblock pre:not(.mermaid)").forEach((pre) => {
  const button = document.createElement("button");
  button.className = "copy";
  button.type = "button";
  button.textContent = "Copy";
  button.addEventListener("click", async () => {
    await navigator.clipboard.writeText(pre.innerText);
    button.textContent = "Copied";
    setTimeout(() => (button.textContent = "Copy"), 1500);
  });
  pre.parentElement.append(button);
});

// A click on the link after a section title also copies it. Hidden from
// screen readers like on GitHub, since a label would join the heading's name.
document.querySelectorAll(".adoc .anchor").forEach((link) => {
  link.setAttribute("aria-hidden", "true");
  link.tabIndex = -1;
  link.addEventListener("click", async () => {
    // Without the query, a shared ?q=term link would open the search again.
    await navigator.clipboard.writeText(location.origin + location.pathname + link.hash);
    link.classList.add("copied");
    setTimeout(() => link.classList.remove("copied"), 1500);
  });
});

// Highlight the table of contents entry of the section being read.
const tocLinks = [...document.querySelectorAll(".toc a")];
if (tocLinks.length) {
  const byId = new Map(tocLinks.map((link) => [decodeURIComponent(link.hash.slice(1)), link]));
  const headings = [...byId.keys()].map((id) => document.getElementById(id)).filter(Boolean);
  const update = () => {
    let current = headings[0];
    for (const heading of headings) {
      if (heading.getBoundingClientRect().top < 120) current = heading;
    }
    tocLinks.forEach((link) => link.classList.toggle("active", byId.get(current?.id) === link));
  };
  document.addEventListener("scroll", update, { passive: true });
  update();
}

// highlight.js and Mermaid load only on pages that need them.
window.addEventListener("load", () => {
  window.hljs?.highlightAll();
  window.mermaid?.initialize({
    startOnLoad: false,
    theme: "base",
    themeVariables: {
      darkMode: true,
      background: "#16191e",
      primaryColor: "#1a1d23",
      primaryBorderColor: "#3a86f0",
      primaryTextColor: "#f0f3f6",
      lineColor: "#969fab",
      fontFamily: "system-ui, sans-serif",
    },
  });
  window.mermaid?.run({ querySelector: "pre.mermaid" });
});
