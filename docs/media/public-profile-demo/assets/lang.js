(function () {
  var lang = "en";
  try {
    var q = new URLSearchParams(location.search).get("lang");
    function ok(v) { return v === "en" || v === "zh-CN"; }
    if (ok(q)) lang = q;
    else {
      var saved = localStorage.getItem("wt-lang");
      if (ok(saved)) lang = saved;
    }
  } catch (e) {}
  document.documentElement.lang = lang;
  if (lang !== "en") document.documentElement.setAttribute("data-i18n-pending", "1");
})();
