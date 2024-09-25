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
        if (matchers[i].hasAttribute("data-arg-builder-registered")) continue;
        
        let arg = matchers[i].parentElement.querySelector(".matcher-arg");
        let property = matchers[i].parentElement.querySelector(".matcher-property");
        let operator = matchers[i].parentElement.querySelector(".matcher-operator");
        let value = matchers[i].parentElement.querySelector(".matcher-value");
        let builder = argBuilder(arg, property, operator, value);

        var lastTimeout = undefined;
        let builderAndSubmit = function() {
            builder();
            if (typeof lastTimeout !== undefined) clearTimeout(lastTimeout);
            lastTimeout = setTimeout(function() {
                document.getElementById("trigger-submit").dispatchEvent(new Event("input"))
            }, 1000);
        }
        matchers[i].addEventListener("input", builderAndSubmit);
        builder();

        matchers[i].setAttribute("data-arg-builder-registered", "");

    }
}

function registerCopyButton() {
    let button = document.getElementById("url-copy");
    if (button == null) return;
    if (button.hasAttribute("data-url-copy-registered")) return;
    button.addEventListener("click", function() {
        copyText = document.getElementById("url-box");

        copyText.select();
        copyText.setSelectionRange(0, 99999); // For mobile devices

        navigator.clipboard.writeText(copyText.value); // This wont work in plain HTTP
    })
    button.setAttribute("data-url-copy-registered", "");
}

function registerSubmitById(id, timeout) {
    elem = document.getElementById(id, timeout);
    if (elem.hasAttribute("data-submit-registered")) return;
    var lastTimeout = undefined;
    elem.addEventListener("input", function() {
        if (typeof lastTimeout !== undefined) clearTimeout(lastTimeout);
        lastTimeout = setTimeout(function() {
            document.getElementById("trigger-submit").dispatchEvent(new Event("input"));
        }, timeout);
    });
    elem.setAttribute("data-submit-registered", "");
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
        registerSubmitById("date-pick-year", 1000);
        registerSubmitById("date-pick-month", 0);
    });

    registerArgBuilders();
};
