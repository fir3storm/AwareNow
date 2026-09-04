/**
 * AwareNowTracker - client-side behavioral tracking library.
 *
 * Tracks page entry/exit, form interactions, and tab visibility,
 * sending batched events to the server via navigator.sendBeacon.
 *
 * Usage:
 *   AwareNowTracker.init('result-id-here');
 */
(function (global) {
    'use strict';

    var ENDPOINT = '/behavior-events';
    var BATCH_INTERVAL = 5000; // ms between automatic flushes
    var MAX_BATCH = 50;        // max events per beacon send

    var rid = null;
    var sessionId = null;
    var eventQueue = [];
    var flushTimer = null;
    var pageEnterTime = Date.now();

    // ---------------------------------------------------------------
    // Utility helpers
    // ---------------------------------------------------------------

    function generateSessionId() {
        // Prefer the crypto API; fall back to Math.random
        if (global.crypto && global.crypto.getRandomValues) {
            var buf = new Uint8Array(8);
            global.crypto.getRandomValues(buf);
            var s = '';
            for (var i = 0; i < buf.length; i++) {
                s += buf[i].toString(16);
            }
            return s;
        }
        return 'sess-' + Math.random().toString(36).substring(2) +
               Date.now().toString(36);
    }

    function uuid() {
        if (global.crypto && global.crypto.randomUUID) {
            return global.crypto.randomUUID();
        }
        // RFC4122 v4-ish fallback
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            var r = Math.random() * 16 | 0;
            var v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    function serialize(obj) {
        try {
            return JSON.stringify(obj);
        } catch (e) {
            return '{}';
        }
    }

    // ---------------------------------------------------------------
    // Event queue
    // ---------------------------------------------------------------

    function enqueue(event) {
        event._eid = uuid();
        event._ts = Date.now();
        eventQueue.push(event);

        if (eventQueue.length >= MAX_BATCH) {
            flush();
        }
    }

    function flush() {
        if (eventQueue.length === 0) {
            return;
        }

        var payload = serialize({
            rid: rid,
            session_id: sessionId,
            user_agent: global.navigator.userAgent,
            events: eventQueue.splice(0, eventQueue.length)
        });

        if (global.navigator.sendBeacon) {
            global.navigator.sendBeacon(ENDPOINT, payload);
        } else {
            // Fallback for browsers without sendBeacon
            var xhr = new XMLHttpRequest();
            xhr.open('POST', ENDPOINT, false);
            xhr.setRequestHeader('Content-Type', 'application/json');
            try { xhr.send(payload); } catch (e) { /* swallow */ }
        }
    }

    function startFlushTimer() {
        if (flushTimer !== null) {
            clearInterval(flushTimer);
        }
        flushTimer = setInterval(flush, BATCH_INTERVAL);
    }

    // ---------------------------------------------------------------
    // Event builders
    // ---------------------------------------------------------------

    function pageEnter() {
        enqueue({
            type: 'page_enter',
            url: global.location.href,
            referrer: document.referrer || '',
            screen_width: global.screen ? global.screen.width : 0,
            screen_height: global.screen ? global.screen.height : 0,
            viewport_width: global.innerWidth || document.documentElement.clientWidth || 0,
            viewport_height: global.innerHeight || document.documentElement.clientHeight || 0,
        });
    }

    function pageExit() {
        var duration = Date.now() - pageEnterTime;
        enqueue({
            type: 'page_exit',
            url: global.location.href,
            duration_ms: duration,
        });
    }

    function tabHidden() {
        enqueue({
            type: 'tab_hidden',
            url: global.location.href,
        });
    }

    function tabVisible() {
        enqueue({
            type: 'tab_visible',
            url: global.location.href,
        });
    }

    function fieldFocus(target) {
        enqueue({
            type: 'field_focus',
            form_id: getFormId(target),
            field_name: target.name || '',
            field_type: target.type || '',
        });
    }

    function fieldBlur(target) {
        enqueue({
            type: 'field_blur',
            form_id: getFormId(target),
            field_name: target.name || '',
            field_type: target.type || '',
            value_length: target.value ? target.value.length : 0,
        });
    }

    function fieldInput(target) {
        enqueue({
            type: 'field_input',
            form_id: getFormId(target),
            field_name: target.name || '',
            field_type: target.type || '',
            value_length: target.value ? target.value.length : 0,
        });
    }

    function formSubmit(target) {
        var fields = [];
        var elements = target.elements || [];
        for (var i = 0; i < elements.length; i++) {
            var el = elements[i];
            if (el.name) {
                fields.push({
                    name: el.name,
                    type: el.type || '',
                    value_length: el.value ? el.value.length : 0,
                });
            }
        }
        enqueue({
            type: 'form_submit',
            form_id: target.id || '',
            form_action: target.action || '',
            form_method: target.method || '',
            fields: fields,
        });
    }

    // ---------------------------------------------------------------
    // Helpers
    // ---------------------------------------------------------------

    function getFormId(el) {
        var form = el && el.form;
        if (form) {
            return form.id || form.name || form.action || '';
        }
        return '';
    }

    // ---------------------------------------------------------------
    // Event listeners (delegated from document)
    // ---------------------------------------------------------------

    function onFocusIn(e) {
        var t = e.target;
        if (!t || !t.tagName) { return; }
        var tag = t.tagName.toLowerCase();
        if (tag === 'input' || tag === 'textarea' || tag === 'select') {
            fieldFocus(t);
        }
    }

    function onFocusOut(e) {
        var t = e.target;
        if (!t || !t.tagName) { return; }
        var tag = t.tagName.toLowerCase();
        if (tag === 'input' || tag === 'textarea' || tag === 'select') {
            fieldBlur(t);
        }
    }

    function onInput(e) {
        var t = e.target;
        if (!t || !t.tagName) { return; }
        var tag = t.tagName.toLowerCase();
        if (tag === 'input' || tag === 'textarea' || tag === 'select') {
            fieldInput(t);
        }
    }

    function onSubmit(e) {
        if (e.target && e.target.tagName && e.target.tagName.toLowerCase() === 'form') {
            formSubmit(e.target);
        }
    }

    function onVisibilityChange() {
        if (document.hidden) {
            tabHidden();
        } else {
            tabVisible();
        }
    }

    function onBeforeUnload() {
        pageExit();
        flush(); // Synchronous flush — sendBeacon is reliable on unload
    }

    function onPageHide(e) {
        // pagehide is more reliable than beforeunload on mobile
        if (!e.persisted) {
            pageExit();
            flush();
        }
    }

    function bindEvents() {
        // Use capture phase so events are caught even if stopPropagation is called
        document.addEventListener('focusin', onFocusIn, true);
        document.addEventListener('focusout', onFocusOut, true);
        document.addEventListener('input', onInput, true);
        document.addEventListener('submit', onSubmit, true);

        // Visibility
        document.addEventListener('visibilitychange', onVisibilityChange, false);

        // Page exit
        global.addEventListener('beforeunload', onBeforeUnload, false);
        global.addEventListener('pagehide', onPageHide, false);
    }

    // ---------------------------------------------------------------
    // Public API
    // ---------------------------------------------------------------

    var AwareNowTracker = {
        /**
         * Initialize the tracker.
         * @param {string} resultId - The campaign result ID (rid) to associate events with.
         */
        init: function (resultId) {
            if (!resultId) {
                if (global.console && global.console.warn) {
                    global.console.warn('AwareNowTracker.init() called without a result ID.');
                }
                return;
            }

            rid = resultId;
            sessionId = generateSessionId();

            pageEnter();
            bindEvents();
            startFlushTimer();
        },

        /**
         * Manually flush the event queue (e.g., before an AJAX navigation).
         */
        flush: flush,

        /**
         * Get the current session ID.
         * @returns {string|null}
         */
        getSessionId: function () {
            return sessionId;
        },
    };

    global.AwareNowTracker = AwareNowTracker;

})(typeof window !== 'undefined' ? window : this);
