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
