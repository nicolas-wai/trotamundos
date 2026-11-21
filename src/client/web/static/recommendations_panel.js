// [ADR-23] Panel de recomendaciones asíncrono: request independiente del dashboard.
// Se inicia DESPUÉS del primer render de las reservas y nunca las bloquea: muestra su propio estado de
// carga y, si falla o demora (el servidor corta a los 3 s, QA17), su propio estado de error con "Reintentar".
// [ADR-25] Aceptar / rechazar / ajustar el presupuesto envía feedback human-in-the-loop; la siguiente
// recomendación ya lo usa (QA18), por eso tras enviarlo se vuelve a pedir la lista.
// Todo texto del servidor se inserta con textContent (nunca innerHTML).
(function (global) {
  'use strict';

  var CATEGORY_LABELS = { ALOJAMIENTO: 'Alojamiento', TRANSPORTE: 'Transporte', RUTA: 'Ruta' };

  // api = { request(method, path, body, accept), notify(mensaje) }: lo entrega el runtime (Bearer y refresh incluidos).
  var api = null;
  var section = null;
  var generation = 0; // descarta respuestas de pedidos viejos (reintentos, recargas del dashboard)

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) { node.className = className; }
    if (text !== undefined) { node.textContent = text; }
    return node;
  }

  function clear() { while (section.firstChild) { section.removeChild(section.firstChild); } }

  function heading() {
    var header = el('header', 'panel-header');
    header.appendChild(el('h2', null, 'Recomendaciones para ti'));
    return header;
  }

  function renderLoading() {
    clear();
    section.appendChild(heading());
    var p = el('p', 'loading', 'Buscando recomendaciones…');
    p.setAttribute('role', 'status');
    section.appendChild(p);
  }

  function renderError() {
    clear();
    section.appendChild(heading());
    var p = el('p', 'form-error', 'No pudimos cargar las recomendaciones por ahora. Tus reservas no se ven afectadas.');
    p.setAttribute('role', 'alert');
    section.appendChild(p);
    var retry = el('button', 'secondary', 'Reintentar');
    retry.type = 'button';
    retry.addEventListener('click', load);
    section.appendChild(retry);
  }

  function price(item) {
    var m = item.metadatos || {};
    return typeof m.precio === 'number' ? m.precio : null;
  }

  function describe(item) {
    var m = item.metadatos || {};
    var parts = [CATEGORY_LABELS[item.categoria] || item.categoria];
    if (m.destino) { parts.push(m.destino); }
    if (m.estrellas) { parts.push(m.estrellas + '★'); }
    if (price(item) !== null) { parts.push('USD ' + price(item)); }
    parts.push(item.proveedor);
    return parts.join(' · ');
  }

  function renderList(items) {
    clear();
    section.appendChild(heading());
    if (!items.length) {
      section.appendChild(el('p', 'empty', 'Por ahora no hay recomendaciones que coincidan con tus preferencias.'));
      return;
    }
    var list = el('ul', 'recommendations');
    items.forEach(function (item) { list.appendChild(renderItem(item)); });
    section.appendChild(list);
  }

  function renderItem(item) {
    var li = el('li', 'recommendation');
    li.dataset.id = item.idRecomendacion;
    li.appendChild(el('span', 'title', item.titulo));
    li.appendChild(el('span', 'meta', describe(item)));

    var actions = el('span', 'actions');
    [['ACEPTAR', 'Me interesa'], ['RECHAZAR', 'No me interesa']].forEach(function (a) {
      var b = el('button', 'secondary', a[1]);
      b.type = 'button';
      b.addEventListener('click', function () { sendFeedback(item, a[0], undefined); });
      actions.appendChild(b);
    });
    var adjust = el('button', 'secondary', 'Ajustar presupuesto');
    adjust.type = 'button';
    adjust.addEventListener('click', function () { toggleAdjustForm(li, item); });
    actions.appendChild(adjust);
    li.appendChild(actions);
    return li;
  }

  function toggleAdjustForm(li, item) {
    var existing = li.querySelector('form');
    if (existing) { li.removeChild(existing); return; }
    var form = el('form', 'adjust-form');
    var label = el('label', null, 'Presupuesto máximo (USD) ');
    var input = document.createElement('input');
    input.type = 'number';
    input.min = '1';
    input.step = '1';
    input.required = true;
    if (price(item) !== null) { input.value = String(Math.max(1, Math.floor(price(item)) - 1)); }
    label.appendChild(input);
    form.appendChild(label);
    var submit = el('button', null, 'Aplicar');
    submit.type = 'submit';
    form.appendChild(submit);
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var max = Number(input.value);
      if (!(max > 0)) { return; }
      sendFeedback(item, 'AJUSTAR', { presupuestoMax: max });
    });
    li.appendChild(form);
    input.focus();
  }

  function sendFeedback(item, accion, params) {
    var body = { accion: accion };
    if (params) { body.parametrosAjustados = params; }
    return api.request('POST', '/recomendaciones/' + encodeURIComponent(item.idRecomendacion) + '/feedback', body)
      .then(function (resp) {
        if (!resp.ok) { api.notify('No se pudo registrar tu respuesta. Intenta de nuevo.'); return; }
        return load(); // la siguiente recomendación ya usa el feedback [QA-18]
      })
      .catch(function () { api.notify('Sin conexión con el servidor.'); });
  }

  // load es la request independiente de recomendaciones: nunca toca el resto del dashboard. [ADR-23]
  function load() {
    var mine = ++generation;
    renderLoading();
    return api.request('GET', '/recomendaciones', undefined, 'application/json')
      .then(function (resp) {
        if (mine !== generation || !section.isConnected) { return; }
        if (!resp.ok) { renderError(); return; }
        return resp.json().then(function (body) {
          if (mine !== generation || !section.isConnected) { return; }
          renderList(body.recomendaciones || []);
        });
      })
      .catch(function () { if (mine === generation && section.isConnected) { renderError(); } });
  }

  // init agrega el panel debajo de las reservas ya renderizadas y dispara su request, sin esperarla.
  function init(container, runtimeApi) {
    api = runtimeApi;
    section = el('section', 'card recommendations-panel');
    section.id = 'recommendations-panel';
    section.setAttribute('aria-live', 'polite');
    container.appendChild(section);
    load();
  }

  global.RecommendationsPanel = { init: init, reload: function () { return section ? load() : undefined; } };
})(window);
