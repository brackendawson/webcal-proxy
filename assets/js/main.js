htmx.config.selfRequestsOnly = true;
htmx.config.includeIndicatorStyles = false;

function argBuilder(arg, property, operator, value) {
    return function() {
        if (value.value == "") {
            arg.removeAttribute("name");
            arg.value="";
            return;
        }

        arg.name = operator.value;
        arg.value = property.value + "=" + value.value;
    };
}

function registerArgBuilders() {
    let matchers = document.getElementsByClassName("matcher");
    for (let i = 0; i < matchers.length; i++) {
        if (matchers[i].classList.contains("arg-builder-registered")) continue;
        
        let arg = matchers[i].parentElement.querySelector(".matcher-arg");
        let property = matchers[i].parentElement.querySelector(".matcher-property");
        let operator = matchers[i].parentElement.querySelector(".matcher-operator");
        let value = matchers[i].parentElement.querySelector(".matcher-value");
        let builder = argBuilder(arg, property, operator, value);

        matchers[i].addEventListener("input", builder);
        builder();

        matchers[i].classList.add("arg-builder-registered");

    }
}

function registerCopyButton() {
    let button = document.getElementById("url-copy");
    if (button == null) return;
    if (button.classList.contains("url-copy-registered")) return;
    button.addEventListener("click", function() {
        copyText = document.getElementById("url-box");

        copyText.select();
        copyText.setSelectionRange(0, 99999); // For mobile devices

        navigator.clipboard.writeText(copyText.value); // This wont work in plain HTTP
    })
    button.classList.add("url-copy-registered");
}

window.onload = function() {
    document.getElementById("config-form").addEventListener("submit", function(event) {
            // prevent form submission, only HTMX allowed
            event.preventDefault();
    });

    document.getElementById("user-tz").value = Intl.DateTimeFormat().resolvedOptions().timeZone;

    document.body.addEventListener("htmx:afterSettle", function() {
        registerCopyButton();
        registerArgBuilders();
    });

    registerArgBuilders();
};
