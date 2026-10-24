// The panel's theme before the first paint, so a dark theme never flashes light. A plain
// file, not inline: the panel's CSP allows scripts from itself only. The lists are the
// ones in src/components/theme.tsx, which takes over once the app runs.
(function () {
  var dark = ["midnight", "graphite", "abyss", "ember", "plum", "moss", "terminal", "nord", "mocha", "tokyo", "dracula", "aurora", "cosmos"];
  var all = ["mirai", "ocean", "sakura", "forest", "latte", "snow", "dawn", "dune"].concat(dark);
  var theme = "mirai";
  try {
    var saved = localStorage.getItem("mirai-theme");
    if (saved === "system") {
      theme = window.matchMedia && matchMedia("(prefers-color-scheme: dark)").matches ? "midnight" : "mirai";
    } else if (all.indexOf(saved) >= 0) {
      theme = saved;
    }
  } catch (e) {
    // Storage blocked: the default theme.
  }
  var root = document.documentElement;
  root.setAttribute("data-theme", theme);
  root.setAttribute("data-tone", dark.indexOf(theme) >= 0 ? "dark" : "light");
})();
