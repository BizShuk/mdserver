// Progressive enhancement for rendered pages: mermaid diagrams, code copy
// buttons, a mobile nav toggle and table-of-contents highlighting. Everything
// here is optional — the page is fully readable without it.
(function () {
  "use strict";

  var prefersDark = window.matchMedia("(prefers-color-scheme: dark)");

  // reveal un-rendered diagrams so a mermaid failure degrades to source text
  // rather than a blank gap.
  function revealDiagrams() {
    document.querySelectorAll("pre.mermaid:not([data-processed])").forEach(function (pre) {
      pre.setAttribute("data-processed", "true");
    });
  }

  function initMermaid() {
    var diagrams = document.querySelectorAll("pre.mermaid");
    if (!diagrams.length) return;
    if (!window.mermaid) {
      revealDiagrams();
      return;
    }
    window.mermaid.initialize({
      startOnLoad: false,
      theme: prefersDark.matches ? "dark" : "default",
      securityLevel: "strict",
      fontFamily: "inherit",
    });
    Promise.resolve(window.mermaid.run({ nodes: diagrams })).catch(revealDiagrams);
  }

  function initCopyButtons() {
    var blocks = document.querySelectorAll(".markdown-body pre:not(.mermaid)");
    blocks.forEach(function (pre) {
      var button = document.createElement("button");
      button.className = "code-copy";
      button.type = "button";
      button.textContent = "Copy";
      button.addEventListener("click", function () {
        var code = pre.querySelector("code");
        var text = (code || pre).innerText;
        navigator.clipboard.writeText(text).then(
          function () {
            button.textContent = "Copied";
            setTimeout(function () {
              button.textContent = "Copy";
            }, 1500);
          },
          function () {
            button.textContent = "Failed";
          }
        );
      });
      pre.appendChild(button);
    });
  }

  function initNavToggle() {
    var toggle = document.querySelector(".nav-toggle");
    var sidebar = document.getElementById("sidebar");
    if (!toggle || !sidebar) return;
    toggle.addEventListener("click", function () {
      var open = sidebar.classList.toggle("open");
      toggle.setAttribute("aria-expanded", String(open));
    });
  }

  function initTocHighlight() {
    var links = document.querySelectorAll(".toc a");
    if (!links.length || !("IntersectionObserver" in window)) return;

    var byId = {};
    var targets = [];
    links.forEach(function (link) {
      var id = decodeURIComponent(link.getAttribute("href").slice(1));
      var heading = document.getElementById(id);
      if (!heading) return;
      byId[id] = link;
      targets.push(heading);
    });

    var visible = new Set();
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) visible.add(entry.target.id);
          else visible.delete(entry.target.id);
        });
        var first = targets.find(function (t) {
          return visible.has(t.id);
        });
        links.forEach(function (link) {
          link.classList.remove("active");
        });
        if (first && byId[first.id]) byId[first.id].classList.add("active");
      },
      { rootMargin: "-72px 0px -70% 0px" }
    );
    targets.forEach(function (t) {
      observer.observe(t);
    });
  }

  function scrollCurrentNavIntoView() {
    var current = document.querySelector(".nav-list a.current");
    if (current && current.scrollIntoView) {
      current.scrollIntoView({ block: "nearest" });
    }
  }

  function init() {
    initMermaid();
    initCopyButtons();
    initNavToggle();
    initTocHighlight();
    scrollCurrentNavIntoView();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
