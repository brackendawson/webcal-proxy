htmx.config.selfRequestsOnly = true;
htmx.config.includeIndicatorStyles = false;

window.onload = function() {
    document.getElementById("user-tz").value = Intl.DateTimeFormat().resolvedOptions().timeZone;
};
