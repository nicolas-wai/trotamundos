// [ADR-18] Gestor de estado optimista: aplica el cambio local al instante y lo revierte si el backend
// responde distinto de 2xx. Cada operación devuelve un "ticket" con commit() y rollback().
(function (global) {
  'use strict';

  var TYPE_LABELS = { VUELO: 'Vuelo', ALOJAMIENTO: 'Alojamiento', AUTO: 'Auto', TREN: 'Tren', BUS: 'Bus' };
  var STATUS_LABELS = { CONFIRMADO: 'Confirmado', CANCELADO: 'Cancelado', MODIFICADO: 'Modificado' };
  var ORIGIN_LABELS = { MANUAL: 'Manual', EMAIL: 'Correo', AGENCIA: 'Agencia', VIAJES: 'Sistema de viajes' };
  var tempSeq = 0;

  function formatDateTime(iso) {
    var d = new Date(iso);
    if (isNaN(d.getTime())) { return ''; }
    return d.toLocaleString('es', { dateStyle: 'short', timeStyle: 'short' });
  }

  function node(tag, className, text) {
    var n = document.createElement(tag);
    if (className) { n.className = className; }
    if (text !== undefined) { n.textContent = text; }
    return n;
  }

  function actionButton(action, label) {
    var b = node('button', '', label);
    b.type = 'button';
    b.setAttribute('data-action', action);
    return b;
  }

  // Misma estructura que la plantilla SSR "arrangement_item" (src/client/web/templates/dashboard.html).
  function buildItem(a, pending) {
    var li = node('li', 'arrangement' + (pending ? ' pending' : ''));
    li.dataset.id = a.id;
    li.dataset.type = a.type;
    li.dataset.title = a.title;
    li.dataset.start = a.start;
    li.dataset.end = a.end || '';
    li.dataset.status = a.status || 'CONFIRMADO';
    li.dataset.origin = a.origin || 'MANUAL';
    li.appendChild(node('span', 'badge', TYPE_LABELS[a.type] || a.type));
    li.appendChild(node('span', 'title', a.title));
    var time = node('time', '', formatDateTime(a.start));
    time.setAttribute('datetime', a.start);
    li.appendChild(time);
    li.appendChild(node('span', 'status status-' + li.dataset.status, STATUS_LABELS[li.dataset.status] || li.dataset.status));
    li.appendChild(node('span', 'origin', ORIGIN_LABELS[li.dataset.origin] || li.dataset.origin));
    var actions = node('span', 'actions');
    actions.appendChild(actionButton('detail', 'Ver detalle'));
    actions.appendChild(actionButton('edit', 'Editar'));
    actions.appendChild(actionButton('delete', 'Eliminar'));
    li.appendChild(actions);
    var detail = node('div', 'detail');
    detail.hidden = true;
    li.appendChild(detail);
    if (pending) { li.appendChild(node('span', 'saving', 'guardando…')); }
    return li;
  }

  function markSynced(li) {
    li.classList.remove('pending');
    var saving = li.querySelector(':scope > .saving');
    if (saving) { saving.remove(); }
  }

  function clearEmpty(ul) {
    var empty = ul.querySelector(':scope > .empty');
    if (empty) { empty.remove(); }
  }

  function ensureEmpty(ul) {
    if (!ul.querySelector(':scope > .arrangement') && !ul.querySelector(':scope > .empty')) {
      ul.appendChild(node('li', 'empty', 'Aún no hay reservas en este viaje.'));
    }
  }

  function OptimisticState(root) { this.root = root; }

  OptimisticState.prototype.findItem = function (id) {
    var items = this.root.querySelectorAll('li.arrangement');
    for (var i = 0; i < items.length; i++) {
      if (items[i].dataset.id === id) { return items[i]; }
    }
    return null;
  };

  OptimisticState.prototype.tripList = function (tripId) {
    var trips = this.root.querySelectorAll('.trip');
    for (var i = 0; i < trips.length; i++) {
      if (trips[i].getAttribute('data-trip-id') === tripId) { return trips[i].querySelector('ul.arrangements'); }
    }
    return null;
  };

  function reorder(state) {
    if (global.VisualPreferences) { global.VisualPreferences.applyOrder(state.root); }
  }

  // Alta: muestra la reserva ya, con un id temporal que commit() reemplaza por el real.
  OptimisticState.prototype.create = function (tripId, data) {
    var state = this;
    var ul = this.tripList(tripId);
    var li = buildItem({ id: 'tmp-' + (++tempSeq), type: data.type, title: data.title, start: data.start, end: data.end }, true);
    clearEmpty(ul);
    ul.appendChild(li);
    reorder(state);
    return {
      commit: function (realId) { li.dataset.id = realId; markSynced(li); },
      rollback: function () { li.remove(); ensureEmpty(ul); }
    };
  };

  // Edición: reemplaza la fila en el lugar y guarda la anterior para revertir.
  OptimisticState.prototype.update = function (id, data) {
    var state = this;
    var previous = this.findItem(id);
    if (!previous) { return null; }
    var next = buildItem({
      id: id, type: data.type, title: data.title, start: data.start, end: data.end,
      status: previous.dataset.status, origin: previous.dataset.origin
    }, true);
    previous.replaceWith(next);
    reorder(state);
    return {
      commit: function () { markSynced(next); },
      rollback: function () { next.replaceWith(previous); reorder(state); }
    };
  };

  // Baja: quita la fila ya y la restaura en su posición si el backend la rechaza.
  OptimisticState.prototype.remove = function (id) {
    var li = this.findItem(id);
    if (!li) { return null; }
    var parent = li.parentNode;
    var nextSibling = li.nextSibling;
    li.remove();
    ensureEmpty(parent);
    return {
      commit: function () { /* nada que confirmar */ },
      rollback: function () { clearEmpty(parent); parent.insertBefore(li, nextSibling); }
    };
  };

  global.OptimisticState = OptimisticState;
  global.OptimisticState.formatDateTime = formatDateTime;
})(window);
