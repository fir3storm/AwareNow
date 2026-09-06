/*
 * AwareNow spam-reporting event handler.
 *
 * IMPORTANT: This file must NOT contain any `import`/`export` statement.
 * Per Microsoft's docs, "imports aren't currently supported in the
 * JavaScript file that contains the code to handle the spam-reporting
 * event" on classic Outlook on Windows, which loads this file as a plain
 * classic script (see the <Override type="javascript" .../> in manifest.xml
 * and the <script> tag in commands.html). Keep this file self-contained.
 *
 * Also per Microsoft's docs: Office.onReady()/Office.initialize do NOT run
 * before this handler fires when it is triggered by the SpamReporting
 * event. Do not rely on any startup/init code having executed first — every
 * function here must work correctly cold, with no shared module state.
 */

// Duplicated from ./extractRid.js (an ES module, tested separately in
// extractRid.test.js) rather than imported, because of the no-imports
// constraint above. Keep this regex in sync with extractRid.js by hand if
// the pattern ever changes.
function extractRid(text) {
  if (typeof text !== "string") {
    return null;
  }
  var match = /[?&]rid=([A-Za-z0-9]{7})\b/.exec(text);
  return match ? match[1] : null;
}

/**
 * Handles Outlook's SpamReporting event, fired after the user confirms the
 * preprocessing dialog for the "Report to AwareNow" button.
 *
 * @param {Office.AddinCommands.Event} event
 */
function onSpamReport(event) {
  var serverUrl;
  try {
    serverUrl = Office.context.roamingSettings.get("awarenowServerUrl");
  } catch (settingsErr) {
    serverUrl = null;
  }

  if (!serverUrl) {
    event.completed({
      moveItemTo: Office.MailboxEnums.MoveSpamItemTo.NoMove,
      showPostProcessingDialog: {
        title: "AwareNow",
        description:
          "AwareNow server URL is not configured for this add-in. An admin " +
          "must run Office.context.roamingSettings.set('awarenowServerUrl', " +
          "'<url>') once (see README.md) before reporting will work.",
      },
    });
    return;
  }

  var item = Office.context.mailbox.item;
  var subject = item.subject || "";

  item.body.getAsync(Office.CoercionType.Text, function (asyncResult) {
    if (asyncResult.status !== Office.AsyncResultStatus.Succeeded) {
      completeWithMessage(
        event,
        "Could not read the message body: " +
          (asyncResult.error && asyncResult.error.message
            ? asyncResult.error.message
            : "unknown error") +
          "."
      );
      return;
    }

    var bodyText = asyncResult.value || "";
    var rid = extractRid(bodyText);

    if (rid) {
      fetch(serverUrl + "/report?rid=" + encodeURIComponent(rid), {
        method: "POST",
        mode: "cors",
      })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("server returned status " + response.status);
          }
          completeWithMessage(event, "Thanks — reported for review.", true);
        })
        .catch(function (err) {
          completeWithMessage(
            event,
            "Failed to report this message: " +
              (err && err.message ? err.message : "network error") +
              "."
          );
        });
      return;
    }

    // No recognizable AwareNow rid: fall back to the generic unknown-report
    // intake endpoint, which requires reporter_email + subject + body_text.
    var reporterEmail =
      (Office.context.mailbox.userProfile &&
        Office.context.mailbox.userProfile.emailAddress) ||
      "";

    var idempotencyKey;
    try {
      idempotencyKey = crypto.randomUUID();
    } catch (uuidErr) {
      // Extremely old WebView2/runtime fallback; still unique enough for a
      // single report attempt's retry window.
      idempotencyKey =
        "awarenow-" + Date.now() + "-" + Math.random().toString(36).slice(2);
    }

    fetch(serverUrl + "/report-unknown", {
      method: "POST",
      mode: "cors",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify({
        reporter_email: reporterEmail,
        subject: subject,
        body_text: bodyText,
      }),
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("server returned status " + response.status);
        }
        completeWithMessage(event, "Thanks — reported for review.", true);
      })
      .catch(function (err) {
        completeWithMessage(
          event,
          "Failed to report this message: " +
            (err && err.message ? err.message : "network error") +
            "."
        );
      });
  });
}

/**
 * Completes the SpamReporting event exactly once, always via NoMove (this is
 * a security-awareness reporting tool for a mix of real and simulated
 * phishing, not actual spam triage, so we never move the message out of the
 * inbox on the user's behalf), surfacing success/failure through the
 * post-processing dialog's description text. There is no first-class
 * "report failed" API distinct from event.completed, so failure is only
 * ever communicated this way (see README's "Known limitations" section).
 *
 * @param {Office.AddinCommands.Event} event
 * @param {string} description
 * @param {boolean} [success]
 */
function completeWithMessage(event, description, success) {
  event.completed({
    moveItemTo: Office.MailboxEnums.MoveSpamItemTo.NoMove,
    showPostProcessingDialog: {
      title: "AwareNow",
      description: description,
    },
  });
}

Office.actions.associate("onSpamReport", onSpamReport);
