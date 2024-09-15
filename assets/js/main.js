htmx.config.selfRequestsOnly = true;
htmx.config.includeIndicatorStyles = false;


window.onload = function() {
    document.getElementById("config-form").addEventListener("submit", function(event) {
            // prevent form submission, only HTMX allowed
            event.preventDefault();
    });

    document.getElementById("user-tz").value = Intl.DateTimeFormat().resolvedOptions().timeZone;

    document.body.addEventListener("htmx:afterSettle", function() {
        button = document.getElementById("url-copy")
        if (button == null) {
            return
        }
        button.addEventListener("click", function() {
            copyText = document.getElementById("url-box");

            copyText.select();
            copyText.setSelectionRange(0, 99999); // For mobile devices

            navigator.clipboard.writeText(copyText.value); // This wont work in plain HTTP
        })
    });
};
