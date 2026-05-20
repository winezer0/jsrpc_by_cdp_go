(function() {
    return demo.regAction("sign", function(resolve, param) {
    try {
        const s = atob(String(param));
        console.log("decode success:", s);
        let res = window.sign.getBaseHeader(JSON.parse(s)).signatureStr;
        resolve(res);
    } catch (error) {
        console.error("sign action failed:", error);
        const x = String(error);
        resolve(x);
    }
    });
})()
