// function downloadMemories(url) {    var parts = url.split("?");    var xhttp = new XMLHttpRequest();    xhttp.open("POST", parts[1], true);    xhttp.onreadystatechange = function() {        if (xhttp.readyState == 4 && xhttp.status == 200) {            var a = document.createElement("a");            a.href = xhttp.responseText;            a.style.display = "none";            document.body.appendChild(a);            a.click();            document.getElementById("mem-info-bar").innerText = "";        } else if (xhttp.readyState == 4 && xhttp.status >= 400) {            document.getElementById("mem-info-bar").innerText = "Oops!                 Something went wrong. Status " + xhttp.status        }    };    xhttp.setRequestHeader("Content-type", "application/x-www-form-urlencoded");    xhttp.send(parts[1]);}

// prettyfied-js
function downloadMemories(url) {
    var parts = url.split("?");
    var xhttp = new XMLHttpRequest();
    xhttp.open("POST", parts[1], true);
    xhttp.onreadystatechange = function() {
        if (xhttp.readyState == 4 && xhttp.status == 200) {
            var a = document.createElement("a");
            a.href = xhttp.responseText;
            a.style.display = "none";
            document.body.appendChild(a);
            a.click();
            document.getElementById("mem-info-bar").innerText = "";
        } else if (xhttp.readyState == 4 && xhttp.status >= 400) {
            document.getElementById("mem-info-bar").innerText = "Oops! Something went wrong. Status " + xhttp.status
        }
    };
    xhttp.setRequestHeader("Content-type", "application/x-www-form-urlencoded");
    xhttp.send(parts[1]);
}