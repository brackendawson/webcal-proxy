htmx.config.selfRequestsOnly = true;
htmx.config.includeIndicatorStyles = false;


window.onload = function() {
    document.getElementById("config-form").addEventListener("submit", function(event) {
            // prevent form submission, only HTMX allowed
            event.preventDefault();
    });

    document.getElementById("user-tz").value = Intl.DateTimeFormat().resolvedOptions().timeZone;
};
