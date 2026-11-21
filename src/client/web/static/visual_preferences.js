// [ADR-19] Gestor de preferencias visuales: orden y tema en LocalStorage (nunca en el servidor).
// Distintas de las preferencias de recomendación, que viven en el perfil [ADR-25].
// Se carga antes del primer render para aplicar el tema guardado sin parpadeo.
(function (global) {
  'use strict';

  var KEY = 'trotamundos.visualPreferences';
  var ORDERS = ['fecha', 'titulo', 'tipo'];
  var THEMES = ['claro', 'oscuro'];
  var DEFAULTS = { order: 'fecha', theme: 'claro' };

  // LocalStorage puede no existir o lanzar (modo privado, datos bloqueados): siempre dentro de try/catch.
  function load() {
    try {
      var raw = global.localStorage.getItem(KEY);
      if (raw) {
        var parsed = JSON.parse(raw);
        return {
          order: ORDERS.indexOf(parsed.order) >= 0 ? parsed.order : DEFAULTS.order,
          theme: THEMES.indexOf(parsed.theme) >= 0 ? parsed.theme : DEFAULTS.theme
        };
      }
    } catch (e) { /* se usan los valores por defecto */ }
    return { order: DEFAULTS.order, theme: DEFAULTS.theme };
  }

  function save(prefs) {
    try { global.localStorage.setItem(KEY, JSON.stringify(prefs)); } catch (e) { /* sin persistencia */ }
  }

  var current = load();

  function applyTheme() {
    document.documentElement.setAttribute('data-theme', current.theme);
  }

  function compare(a, b) {
    var field = current.order === 'titulo' ? 'title' : current.order === 'tipo' ? 'type' : 'start';
    var x = a.dataset[field] || '';
    var y = b.dataset[field] || '';
    if (field === 'title') { return x.localeCompare(y, 'es', { sensitivity: 'base' }); }
    if (x === y) { return 0; }
    return x < y ? -1 : 1; // las fechas ISO UTC ordenan como texto
  }

  // Reordena en el DOM las reservas de cada viaje según la preferencia.
  function applyOrder(root) {
    var lists = (root || document).querySelectorAll('ul.arrangements');
    Array.prototype.forEach.call(lists, function (ul) {
      var items = Array.prototype.filter.call(ul.children, function (li) {
        return li.classList.contains('arrangement');
      });
      items.sort(compare);
      items.forEach(function (li) { ul.appendChild(li); });
    });
  }

  global.VisualPreferences = {
    get: function () { return { order: current.order, theme: current.theme }; },
    setOrder: function (order) {
      if (ORDERS.indexOf(order) < 0) { return; }
      current.order = order;
      save(current);
    },
    toggleTheme: function () {
      current.theme = current.theme === 'claro' ? 'oscuro' : 'claro';
      save(current);
      applyTheme();
    },
    applyTheme: applyTheme,
    applyOrder: applyOrder
  };

  applyTheme();
})(window);
