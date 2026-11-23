// [ADR-13] Pantalla "Mis itinerarios": el dueño crea itinerarios, agrega o quita reservas, decide las solicitudes de
// acceso y revoca accesos; el invitado abre un itinerario por su enlace y, si recibe 403 con puedeSolicitar, pide acceso.
// [ADR-18] Decidir una solicitud, revocar un acceso y quitar una reserva son optimistas: se reflejan al instante y
// se revierten si el backend responde distinto de 2xx.
// Las solicitudes nuevas aparecen sin recargar la página con una consulta periódica corta (el aviso al dueño por un
// canal externo, E7, sigue siendo una limitación del prototipo). Todo texto del servidor entra con textContent.
(function (global) {
  'use strict';

  var DEFAULT_POLL_MS = 5000;

  // api = { request(method, path, body, accept), notify(mensaje), reload() }: lo entrega el runtime (Bearer y refresh incluidos).
  var api = null;
  var root = null;
  var section = null;
  var inflight = {}; // idSolicitud → true mientras su decisión está en vuelo (el sondeo no la toca)
  var pollTimer = null;

  function el(tag, className, text) {
    var n = document.createElement(tag);
    if (className) { n.className = className; }
    if (text !== undefined) { n.textContent = text; }
    return n;
  }

  function button(label, action, secondary) {
    var b = el('button', secondary ? 'secondary' : '', label);
    b.type = 'button';
    b.setAttribute('data-action', action);
    return b;
  }

  function errorText(resp, fallback) {
    return resp.clone().json().then(function (b) { return b.mensaje || fallback; }, function () { return fallback; });
  }

  // fail avisa con el mensaje del servidor; el rollback visual lo hace quien llama.
  function fail(resp, what) {
    if (resp.status === 401) { return Promise.resolve(); } // el runtime ya mostró el login
    return errorText(resp, 'Error del servidor').then(function (m) {
      api.notify('No se pudo ' + what + ': ' + m + '. Se restauró el estado anterior.');
    });
  }

  function offline(what) { api.notify('No se pudo ' + what + ': sin conexión. Se restauró el estado anterior.'); }

  // ---- listas ----------------------------------------------------------------------------------

  function list(article, cls) { return article.querySelector('ul.' + cls); }

  function emptyText(cls) {
    return {
      'itinerary-arrangements': 'Este itinerario aún no tiene reservas.',
      'itinerary-requests': 'No hay solicitudes pendientes.',
      'itinerary-accesses': 'Nadie tiene acceso todavía.'
    }[cls];
  }

  function clearEmpty(ul) { var e = ul.querySelector(':scope > .empty'); if (e) { e.remove(); } }

  function ensureEmpty(ul, cls) {
    if (!ul.querySelector(':scope > li:not(.empty)') && !ul.querySelector(':scope > .empty')) {
      ul.appendChild(el('li', 'empty', emptyText(cls)));
    }
  }

  function requestItem(requestId, travelerId) {
    var li = el('li', 'request');
    li.dataset.requestId = requestId;
    li.dataset.travelerId = travelerId;
    li.appendChild(el('span', 'who', 'Viajero ' + travelerId));
    li.appendChild(document.createTextNode(' '));
    li.appendChild(button('Aceptar', 'accept-request', false));
    li.appendChild(document.createTextNode(' '));
    li.appendChild(button('Rechazar', 'reject-request', true));
    return li;
  }

  function accessItem(travelerId) {
    var li = el('li', 'access');
    li.dataset.travelerId = travelerId;
    li.appendChild(el('span', 'who', 'Viajero ' + travelerId));
    li.appendChild(document.createTextNode(' '));
    li.appendChild(button('Revocar', 'revoke-access', true));
    return li;
  }

  function arrangementItem(id, label) {
    var li = el('li', 'itinerary-arrangement');
    li.dataset.arrangementId = id;
    li.appendChild(el('span', 'title', label));
    li.appendChild(document.createTextNode(' '));
    li.appendChild(button('Quitar', 'remove-itinerary-arrangement', true));
    return li;
  }

  // Quita el nodo y devuelve cómo volver a ponerlo donde estaba (rollback).
  function detach(node, ul, cls) {
    var next = node.nextSibling;
    node.remove();
    ensureEmpty(ul, cls);
    return function restore() {
      clearEmpty(ul);
      ul.insertBefore(node, next && next.parentNode === ul ? next : null);
    };
  }

  // ---- reservas disponibles para agregar -------------------------------------------------------

  // Las opciones salen de las reservas visibles en el dashboard que el itinerario aún no incluye.
  function fillSelect(article) {
    var select = article.querySelector('select[data-role="arrangement-select"]');
    var included = {};
    Array.prototype.forEach.call(article.querySelectorAll('li.itinerary-arrangement'), function (li) { included[li.dataset.arrangementId] = true; });
    var current = select.value;
    while (select.firstChild) { select.removeChild(select.firstChild); }
    Array.prototype.forEach.call(root.querySelectorAll('li.arrangement'), function (li) {
      var id = li.dataset.id;
      if (!id || id.indexOf('tmp-') === 0 || included[id]) { return; }
      var o = el('option', '', li.dataset.title);
      o.value = id;
      select.appendChild(o);
    });
    if (!select.options.length) {
      var none = el('option', '', 'No hay reservas para agregar');
      none.value = '';
      select.appendChild(none);
    }
    if (current) { select.value = current; }
  }

  function fillSelects() {
    Array.prototype.forEach.call(section.querySelectorAll('article.itinerary'), fillSelect);
  }

  // ---- acciones del dueño ----------------------------------------------------------------------

  function createItinerary(form) {
    var name = form.elements.nombre.value.trim();
    if (!name) { return; }
    api.request('POST', '/itinerarios', { nombre: name, idsArreglos: [] }).then(function (resp) {
      if (!resp.ok) { return errorText(resp, 'Error del servidor').then(function (m) { api.notify('No se pudo crear el itinerario: ' + m); }); }
      return api.reload();
    }).catch(function () { api.notify('No se pudo crear el itinerario: sin conexión.'); });
  }

  function deleteItinerary(article) {
    var id = article.dataset.itineraryId;
    api.request('DELETE', '/itinerarios/' + encodeURIComponent(id)).then(function (resp) {
      if (resp.ok) { article.remove(); return; }
      return errorText(resp, 'Error del servidor').then(function (m) { api.notify('No se pudo eliminar el itinerario: ' + m); });
    }).catch(function () { api.notify('No se pudo eliminar el itinerario: sin conexión.'); });
  }

  function addArrangement(article) {
    var select = article.querySelector('select[data-role="arrangement-select"]');
    var id = select.value;
    if (!id) { return; }
    var label = select.options[select.selectedIndex].textContent;
    api.request('POST', '/itinerarios/' + encodeURIComponent(article.dataset.itineraryId) + '/arreglos', { idArreglo: id }).then(function (resp) {
      if (!resp.ok) { return errorText(resp, 'Error del servidor').then(function (m) { api.notify('No se pudo agregar la reserva: ' + m); }); }
      var ul = list(article, 'itinerary-arrangements');
      if (!ul.querySelector('li[data-arrangement-id="' + id + '"]')) { clearEmpty(ul); ul.appendChild(arrangementItem(id, label)); }
      fillSelect(article);
    }).catch(function () { api.notify('No se pudo agregar la reserva: sin conexión.'); });
  }

  function removeArrangement(article, li) {
    var ul = list(article, 'itinerary-arrangements');
    var restore = detach(li, ul, 'itinerary-arrangements');
    fillSelect(article);
    api.request('DELETE', '/itinerarios/' + encodeURIComponent(article.dataset.itineraryId) + '/arreglos/' + encodeURIComponent(li.dataset.arrangementId))
      .then(function (resp) { if (!resp.ok) { restore(); fillSelect(article); return fail(resp, 'quitar la reserva'); } })
      .catch(function () { restore(); fillSelect(article); offline('quitar la reserva'); });
  }

  // Decidir una solicitud [ADR-18]: la solicitud sale de pendientes (y, si se acepta, el viajero pasa a accesos) al
  // instante; ante un no-2xx todo vuelve atrás.
  function decideRequest(article, li, accept) {
    var requestId = li.dataset.requestId;
    var travelerId = li.dataset.travelerId;
    var requests = list(article, 'itinerary-requests');
    var accesses = list(article, 'itinerary-accesses');
    inflight[requestId] = true;
    var restore = detach(li, requests, 'itinerary-requests');
    var added = null;
    if (accept) { clearEmpty(accesses); added = accessItem(travelerId); added.classList.add('pending'); accesses.appendChild(added); }
    var undo = function () {
      restore();
      if (added) { added.remove(); ensureEmpty(accesses, 'itinerary-accesses'); }
    };
    api.request('POST', '/itinerarios/' + encodeURIComponent(article.dataset.itineraryId) + '/solicitudes/' + encodeURIComponent(requestId) + '/decision',
      { decision: accept ? 'ACEPTAR' : 'RECHAZAR' })
      .then(function (resp) {
        if (resp.ok) { if (added) { added.classList.remove('pending'); } return; }
        undo();
        return fail(resp, accept ? 'aceptar la solicitud' : 'rechazar la solicitud');
      })
      .catch(function () { undo(); offline(accept ? 'aceptar la solicitud' : 'rechazar la solicitud'); })
      .then(function () { delete inflight[requestId]; });
  }

  function revokeAccess(article, li) {
    var accesses = list(article, 'itinerary-accesses');
    var restore = detach(li, accesses, 'itinerary-accesses');
    api.request('DELETE', '/itinerarios/' + encodeURIComponent(article.dataset.itineraryId) + '/accesos/' + encodeURIComponent(li.dataset.travelerId))
      .then(function (resp) { if (!resp.ok) { restore(); return fail(resp, 'revocar el acceso'); } })
      .catch(function () { restore(); offline('revocar el acceso'); });
  }

  // ---- solicitudes nuevas sin recargar ---------------------------------------------------------

  function pollItinerary(article) {
    var id = article.dataset.itineraryId;
    return api.request('GET', '/itinerarios/' + encodeURIComponent(id) + '/solicitudes', undefined, 'application/json').then(function (resp) {
      if (!resp.ok) { return; }
      return resp.json().then(function (body) {
        var ul = list(article, 'itinerary-requests');
        if (!ul) { return; }
        var pending = {};
        (body.solicitudes || []).forEach(function (s) {
          pending[s.idSolicitud] = true;
          if (inflight[s.idSolicitud] || ul.querySelector('li[data-request-id="' + s.idSolicitud + '"]')) { return; }
          clearEmpty(ul);
          ul.appendChild(requestItem(s.idSolicitud, s.idViajero));
        });
        Array.prototype.forEach.call(ul.querySelectorAll('li.request'), function (li) {
          if (!pending[li.dataset.requestId] && !inflight[li.dataset.requestId]) { li.remove(); } // decidida desde otro lado
        });
        ensureEmpty(ul, 'itinerary-requests');
      });
    });
  }

  function schedulePoll(ms) {
    pollTimer = setTimeout(function () {
      if (!document.body.contains(section)) { return; } // el dashboard se recargó: este panel ya no existe
      var articles = Array.prototype.slice.call(section.querySelectorAll('article.itinerary'));
      Promise.all(articles.map(function (a) { return pollItinerary(a).catch(function () {}); })).then(function () { schedulePoll(ms); });
    }, ms);
  }

  // ---- vista del invitado ----------------------------------------------------------------------

  function itineraryIdFrom(text) {
    var parts = text.trim().split('/').filter(Boolean);
    return parts.length ? parts[parts.length - 1] : '';
  }

  function guestBox() { return section.querySelector('[data-role="guest-view"]'); }

  function showGuest(nodes) {
    var box = guestBox();
    while (box.firstChild) { box.removeChild(box.firstChild); }
    nodes.forEach(function (n) { box.appendChild(n); });
  }

  function openItinerary(id) {
    api.request('GET', '/itinerarios/' + encodeURIComponent(id), undefined, 'application/json').then(function (resp) {
      if (resp.status === 401) { return; }
      return resp.json().catch(function () { return {}; }).then(function (body) {
        if (resp.ok) { return showGuest(renderShared(body)); }
        if (resp.status === 403) { return showGuest(renderDenied(id, body)); }
        if (resp.status === 404) { return showGuest([el('p', 'form-error', 'Ese itinerario no existe.')]); }
        showGuest([el('p', 'form-error', body.mensaje || 'No se pudo abrir el itinerario.')]);
      });
    }).catch(function () { showGuest([el('p', 'form-error', 'Sin conexión con el servidor.')]); });
  }

  function renderShared(body) {
    var wrap = el('article', 'shared-itinerary');
    wrap.dataset.itineraryId = body.idItinerario;
    wrap.appendChild(el('h3', '', body.nombre));
    var ul = el('ul', 'shared-arrangements');
    (body.arreglos || []).forEach(function (a) {
      ul.appendChild(el('li', '', a.titulo + ' · ' + new Date(a.fechaInicio).toLocaleString('es', { dateStyle: 'short', timeStyle: 'short' })));
    });
    if (!ul.children.length) { ul.appendChild(el('li', 'empty', 'Este itinerario aún no tiene reservas.')); }
    wrap.appendChild(ul);
    return [wrap];
  }

  function renderDenied(id, body) {
    var nodes = [el('p', 'form-error', 'No tienes acceso a este itinerario.')];
    if (body.puedeSolicitar) {
      var b = button('Solicitar acceso', 'request-access', false);
      b.dataset.itineraryId = id;
      nodes.push(b);
    }
    return nodes;
  }

  function requestAccess(id) {
    api.request('POST', '/itinerarios/' + encodeURIComponent(id) + '/solicitudes').then(function (resp) {
      if (resp.ok) {
        var again = button('Volver a intentar', 'reopen-itinerary', true);
        again.dataset.itineraryId = id;
        return showGuest([el('p', 'notice', 'Solicitud enviada. El dueño del itinerario debe aceptarla.'), again]);
      }
      return errorText(resp, 'Error del servidor').then(function (m) { api.notify('No se pudo solicitar el acceso: ' + m); });
    }).catch(function () { api.notify('No se pudo solicitar el acceso: sin conexión.'); });
  }

  // ---- eventos ---------------------------------------------------------------------------------

  function onClick(event) {
    var b = event.target.closest('button[data-action]');
    if (!b || !section.contains(b)) { return; }
    var action = b.getAttribute('data-action');
    var article = b.closest('article.itinerary');
    var li = b.closest('li');
    switch (action) {
      case 'delete-itinerary': return deleteItinerary(article);
      case 'add-itinerary-arrangement': return addArrangement(article);
      case 'remove-itinerary-arrangement': return removeArrangement(article, li);
      case 'accept-request': return decideRequest(article, li, true);
      case 'reject-request': return decideRequest(article, li, false);
      case 'revoke-access': return revokeAccess(article, li);
      case 'request-access': return requestAccess(b.dataset.itineraryId);
      case 'reopen-itinerary': return openItinerary(b.dataset.itineraryId);
    }
  }

  function onSubmit(event) {
    var form = event.target;
    var kind = form.getAttribute('data-form');
    if (!kind) { return; }
    event.preventDefault();
    if (kind === 'create-itinerary') { createItinerary(form); form.reset(); }
    else if (kind === 'open-itinerary') { var id = itineraryIdFrom(form.elements.enlace.value); if (id) { openItinerary(id); } }
  }

  function init(appRoot, deps) {
    if (pollTimer) { clearTimeout(pollTimer); pollTimer = null; }
    root = appRoot;
    api = deps;
    inflight = {};
    section = root.querySelector('[data-role="itineraries"]');
    if (!section) { return; }
    section.addEventListener('click', onClick);
    section.addEventListener('submit', onSubmit);
    // Las reservas del dashboard pueden cambiar sin recargar: se recalculan las opciones al enfocar el selector.
    section.addEventListener('focusin', function (e) {
      if (e.target.matches && e.target.matches('select[data-role="arrangement-select"]')) { fillSelect(e.target.closest('article.itinerary')); }
    });
    fillSelects();
    schedulePoll(global.TROTAMUNDOS_ITINERARY_POLL_MS || DEFAULT_POLL_MS);
  }

  global.ItinerariesPanel = { init: init, fillSelects: function () { if (section) { fillSelects(); } } };
})(window);
