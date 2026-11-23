// [ADR-18] Runtime de cliente mínimo: login, dashboard SSR inyectado, y alta/edición/baja con optimistic update.
// Todas las llamadas llevan Bearer token [ADR-16]. Los textos de UI van en español.
(function (global) {
  'use strict';

  var SESSION_KEY = 'trotamundos.session';
  var session = null; // { accessToken, refreshToken } — solo en sessionStorage, nunca en LocalStorage

  // Campos de "detalle" por tipo; las claves coinciden con el contrato del backend (AD §6.2, detalle por tipo).
  var DETAIL_FIELDS = {
    VUELO: [['aerolinea', 'Aerolínea'], ['numeroVuelo', 'N.º de vuelo'], ['origenIATA', 'Origen (IATA)'], ['destinoIATA', 'Destino (IATA)']],
    ALOJAMIENTO: [['hotel', 'Hotel'], ['direccion', 'Dirección'], ['estrellas', 'Estrellas', 'number']],
    AUTO: [['empresa', 'Empresa'], ['retiro', 'Lugar de retiro'], ['devolucion', 'Lugar de devolución']],
    TREN: [['operador', 'Operador'], ['origen', 'Origen'], ['destino', 'Destino']],
    BUS: [['operador', 'Operador'], ['origen', 'Origen'], ['destino', 'Destino']]
  };

  var $ = function (id) { return document.getElementById(id); };
  var els = {};
  var state = null;
  var editing = null; // { mode: 'create'|'edit', tripId, id }
  var bannerTimer = null;

  // ---- sesión ----------------------------------------------------------------------------------

  function loadSession() {
    try {
      var raw = global.sessionStorage.getItem(SESSION_KEY);
      if (raw) { session = JSON.parse(raw); }
    } catch (e) { session = null; }
  }

  function storeSession(s) {
    session = s;
    try {
      if (s) { global.sessionStorage.setItem(SESSION_KEY, JSON.stringify(s)); }
      else { global.sessionStorage.removeItem(SESSION_KEY); }
    } catch (e) { /* queda solo en memoria */ }
  }

  function showLogin(message) {
    storeSession(null);
    els.app.hidden = true;
    els.app.innerHTML = '';
    els.toolbar.hidden = true;
    els.login.hidden = false;
    if (message) { showLoginError(message); }
  }

  function showLoginError(message) {
    els.loginError.textContent = message;
    els.loginError.hidden = !message;
  }

  function showApp() {
    els.login.hidden = true;
    els.toolbar.hidden = false;
    els.app.hidden = false;
  }

  // ---- red -------------------------------------------------------------------------------------

  function refreshAccessToken() {
    if (!session || !session.refreshToken) { return Promise.resolve(false); }
    return fetch('/token/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken: session.refreshToken })
    }).then(function (resp) {
      if (!resp.ok) { return false; }
      return resp.json().then(function (body) {
        storeSession({ accessToken: body.accessToken, refreshToken: session.refreshToken });
        return true;
      });
    }).catch(function () { return false; });
  }

  // request envía la llamada con Bearer; ante 401 TOKEN_EXPIRADO renueva el access token una vez y reintenta.
  function request(method, path, body, accept, retried) {
    var headers = {};
    if (accept) { headers.Accept = accept; }
    if (body !== undefined) { headers['Content-Type'] = 'application/json'; }
    if (session && session.accessToken) { headers.Authorization = 'Bearer ' + session.accessToken; }
    return fetch(path, { method: method, headers: headers, body: body === undefined ? undefined : JSON.stringify(body) })
      .then(function (resp) {
        if (resp.status !== 401) { return resp; }
        return resp.clone().json().catch(function () { return {}; }).then(function (err) {
          if (err.codigo === 'TOKEN_EXPIRADO' && !retried) {
            return refreshAccessToken().then(function (ok) {
              return ok ? request(method, path, body, accept, true) : resp;
            });
          }
          return resp;
        });
      })
      .then(function (resp) {
        if (resp.status === 401) { showLogin('Tu sesión expiró. Ingresa nuevamente.'); }
        return resp;
      });
  }

  function errorMessage(resp, fallback) {
    return resp.clone().json().then(function (body) { return body.mensaje || fallback; }, function () { return fallback; });
  }

  function showBanner(message) {
    els.banner.textContent = message;
    els.banner.hidden = false;
    if (bannerTimer) { clearTimeout(bannerTimer); }
    bannerTimer = setTimeout(function () { els.banner.hidden = true; }, 7000);
  }

  // ---- dashboard -------------------------------------------------------------------------------

  function localizeTimes(root) {
    Array.prototype.forEach.call(root.querySelectorAll('time[datetime]'), function (t) {
      var text = global.OptimisticState.formatDateTime(t.getAttribute('datetime'));
      if (text) { t.textContent = text; }
    });
  }

  // loadDashboard pide el HTML renderizado por el servidor (SSR, payload reducido) y lo inyecta. [ADR-20]
  function loadDashboard() {
    showApp();
    els.app.innerHTML = '<p class="loading">Cargando tus viajes…</p>';
    return request('GET', '/dashboard', undefined, 'text/html').then(function (resp) {
      if (!resp.ok) {
        if (resp.status !== 401) { els.app.innerHTML = '<p class="form-error">No se pudo cargar el dashboard. Intenta de nuevo.</p>'; }
        return;
      }
      return resp.text().then(function (html) {
        els.app.innerHTML = html;
        state = new global.OptimisticState(els.app);
        localizeTimes(els.app);
        global.VisualPreferences.applyOrder(els.app); // preferencias aplicadas antes de mostrar [ADR-19]
        // [ADR-23] Request independiente: no se espera ni bloquea las reservas ya visibles.
        // [ADR-13] "Mis itinerarios": parte del mismo HTML del dashboard; su JS solo lo activa.
        if (global.ItinerariesPanel) { global.ItinerariesPanel.init(els.app, { request: request, notify: showBanner, reload: loadDashboard }); }
        if (global.RecommendationsPanel) { global.RecommendationsPanel.init(els.app, { request: request, notify: showBanner }); }
      });
    }).catch(function () {
      els.app.innerHTML = '<p class="form-error">Sin conexión con el servidor.</p>';
    });
  }

  // ---- autenticación ---------------------------------------------------------------------------

  function credentials() {
    var f = els.loginForm;
    return { email: f.elements.email.value.trim(), password: f.elements.password.value };
  }

  function login(creds) {
    return fetch('/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(creds) })
      .then(function (resp) {
        if (!resp.ok) { return errorMessage(resp, 'No se pudo ingresar.').then(function (m) { showLoginError(m); }); }
        return resp.json().then(function (body) {
          showLoginError('');
          storeSession({ accessToken: body.accessToken, refreshToken: body.refreshToken });
          els.loginForm.reset();
          return loadDashboard();
        });
      })
      .catch(function () { showLoginError('Sin conexión con el servidor.'); });
  }

  function register() {
    var creds = credentials();
    if (!els.loginForm.reportValidity()) { return; }
    fetch('/registro', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(creds) })
      .then(function (resp) {
        if (!resp.ok) { return errorMessage(resp, 'No se pudo crear la cuenta.').then(function (m) { showLoginError(m); }); }
        return login(creds);
      })
      .catch(function () { showLoginError('Sin conexión con el servidor.'); });
  }

  function logout() {
    var refreshToken = session && session.refreshToken;
    var done = function () { showLogin(''); };
    if (!refreshToken) { done(); return; }
    fetch('/logout', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ refreshToken: refreshToken }) })
      .then(done, done);
  }

  // ---- formulario de reservas ------------------------------------------------------------------

  function toLocalInput(iso) {
    var d = new Date(iso);
    if (!iso || isNaN(d.getTime())) { return ''; }
    var p = function (n) { return (n < 10 ? '0' : '') + n; };
    return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + 'T' + p(d.getHours()) + ':' + p(d.getMinutes());
  }

  function fromLocalInput(value) {
    return value ? new Date(value).toISOString() : '';
  }

  function renderDetailFields(type, values) {
    var box = $('detail-fields');
    while (box.children.length > 1) { box.removeChild(box.lastChild); } // conserva <legend>
    (DETAIL_FIELDS[type] || []).forEach(function (f) {
      var label = document.createElement('label');
      label.textContent = f[1] + ' ';
      var input = document.createElement('input');
      input.type = f[2] || 'text';
      if (input.type === 'number') { input.min = '1'; input.max = '5'; }
      input.name = 'detalle.' + f[0];
      if (values && values[f[0]] !== undefined) { input.value = values[f[0]]; }
      label.appendChild(input);
      box.appendChild(label);
    });
  }

  function openDialog(mode, tripId, item, detail) {
    var form = els.form;
    form.reset();
    editing = { mode: mode, tripId: tripId, id: item ? item.dataset.id : null };
    $('dialog-title').textContent = mode === 'edit' ? 'Editar reserva' : 'Agregar reserva';
    if (item) {
      form.elements.tipo.value = item.dataset.type;
      form.elements.titulo.value = item.dataset.title;
      form.elements.fechaInicio.value = toLocalInput(item.dataset.start);
      form.elements.fechaFin.value = toLocalInput(item.dataset.end);
    }
    renderDetailFields(form.elements.tipo.value, detail);
    if (typeof els.dialog.showModal === 'function') { els.dialog.showModal(); } else { els.dialog.setAttribute('open', ''); }
  }

  function closeDialog() {
    if (typeof els.dialog.close === 'function') { els.dialog.close(); } else { els.dialog.removeAttribute('open'); }
  }

  function readForm() {
    var f = els.form.elements;
    var detalle = {};
    Array.prototype.forEach.call(els.form.querySelectorAll('[name^="detalle."]'), function (input) {
      var value = input.value.trim();
      if (value === '') { return; }
      detalle[input.name.slice('detalle.'.length)] = input.type === 'number' ? Number(value) : value;
    });
    var body = {
      idViaje: editing.tripId,
      tipo: f.tipo.value,
      titulo: f.titulo.value.trim(),
      fechaInicio: fromLocalInput(f.fechaInicio.value),
      detalle: detalle
    };
    var end = fromLocalInput(f.fechaFin.value);
    if (end) { body.fechaFin = end; }
    return body;
  }

  // Envía el cambio con optimistic update: refleja ya, y confirma o revierte según el código HTTP. [ADR-18]
  function submitArrangement(event) {
    event.preventDefault();
    var body = readForm();
    var target = editing;
    var data = { type: body.tipo, title: body.titulo, start: body.fechaInicio, end: body.fechaFin };
    var ticket, call;
    if (target.mode === 'edit') {
      ticket = state.update(target.id, data);
      call = request('PUT', '/arreglos/' + encodeURIComponent(target.id), body);
    } else {
      ticket = state.create(target.tripId, data);
      call = request('POST', '/arreglos', body);
    }
    closeDialog();
    if (!ticket) { showBanner('La reserva ya no está en la vista.'); return; }
    settle(call, ticket, 'guardar el cambio', function (resp) {
      return target.mode === 'create' ? resp.json().then(function (b) { ticket.commit(b.idArreglo); }) : ticket.commit();
    });
  }

  function deleteArrangement(item) {
    var id = item.dataset.id;
    if (id.indexOf('tmp-') === 0) { return; } // aún guardándose
    var ticket = state.remove(id);
    settle(request('DELETE', '/arreglos/' + encodeURIComponent(id)), ticket, 'eliminar la reserva', function () { ticket.commit(); });
  }

  // settle: 2xx → onOk (marca sincronizado); cualquier otro resultado → rollback visual y aviso.
  function settle(call, ticket, what, onOk) {
    call.then(function (resp) {
      if (resp.ok) { return onOk(resp); }
      ticket.rollback();
      if (resp.status === 401) { return; } // showLogin ya avisó
      return errorMessage(resp, 'Error del servidor').then(function (m) {
        showBanner('No se pudo ' + what + ': ' + m + '. Se restauró el estado anterior.');
      });
    }).catch(function () {
      ticket.rollback();
      showBanner('No se pudo ' + what + ': sin conexión. Se restauró el estado anterior.');
    });
  }

  // El detalle completo se pide bajo demanda como fragmento HTML. [ADR-17]
  function toggleDetail(item) {
    var box = item.querySelector(':scope > .detail');
    if (!box.hidden) { box.hidden = true; return; }
    if (item.dataset.id.indexOf('tmp-') === 0) { return; }
    box.hidden = false;
    box.textContent = 'Cargando detalle…';
    request('GET', '/arreglos/' + encodeURIComponent(item.dataset.id), undefined, 'text/html').then(function (resp) {
      if (!resp.ok) { box.hidden = true; return errorMessage(resp, 'Error del servidor').then(function (m) { showBanner('No se pudo cargar el detalle: ' + m); }); }
      return resp.text().then(function (html) { box.innerHTML = html; localizeTimes(box); });
    }).catch(function () { box.hidden = true; showBanner('No se pudo cargar el detalle: sin conexión.'); });
  }

  function editArrangement(item) {
    if (item.dataset.id.indexOf('tmp-') === 0) { return; }
    var tripId = item.closest('.trip').getAttribute('data-trip-id');
    request('GET', '/arreglos/' + encodeURIComponent(item.dataset.id), undefined, 'application/json').then(function (resp) {
      if (!resp.ok) { return errorMessage(resp, 'Error del servidor').then(function (m) { showBanner('No se pudo abrir la reserva: ' + m); }); }
      return resp.json().then(function (body) { openDialog('edit', tripId, item, body.detalle || {}); });
    }).catch(function () { showBanner('No se pudo abrir la reserva: sin conexión.'); });
  }

  // ---- eventos ---------------------------------------------------------------------------------

  function onAppClick(event) {
    var button = event.target.closest('[data-action]');
    if (!button || !els.app.contains(button)) { return; }
    var action = button.getAttribute('data-action');
    if (action === 'add') {
      openDialog('create', button.closest('.trip').getAttribute('data-trip-id'), null, null);
      return;
    }
    var item = button.closest('li.arrangement');
    if (!item) { return; }
    if (action === 'detail') { toggleDetail(item); }
    else if (action === 'edit') { editArrangement(item); }
    else if (action === 'delete') { deleteArrangement(item); }
  }

  function init() {
    els = {
      login: $('login-view'), loginForm: $('login-form'), loginError: $('login-error'), app: $('app'),
      toolbar: $('toolbar'), banner: $('banner'), dialog: $('arrangement-dialog'), form: $('arrangement-form')
    };
    $('sort-select').value = global.VisualPreferences.get().order;

    els.loginForm.addEventListener('submit', function (e) { e.preventDefault(); login(credentials()); });
    $('register-button').addEventListener('click', register);
    $('logout-button').addEventListener('click', logout);
    $('theme-toggle').addEventListener('click', function () { global.VisualPreferences.toggleTheme(); });
    $('sort-select').addEventListener('change', function (e) {
      global.VisualPreferences.setOrder(e.target.value);
      global.VisualPreferences.applyOrder(els.app);
    });
    els.app.addEventListener('click', onAppClick);
    els.form.addEventListener('submit', submitArrangement);
    $('dialog-cancel').addEventListener('click', closeDialog);
    els.form.elements.tipo.addEventListener('change', function (e) { renderDetailFields(e.target.value, null); });

    loadSession();
    if (session && session.accessToken) { loadDashboard(); } else { showLogin(''); }
  }

  if (document.readyState === 'loading') { document.addEventListener('DOMContentLoaded', init); } else { init(); }
})(window);
