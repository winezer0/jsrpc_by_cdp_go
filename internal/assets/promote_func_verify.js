(function() {
    try {
        var e = {};
        if (typeof window.sign === 'undefined') {
            return JSON.stringify({success: false, message: 'window.sign is undefined'});
        }
        if (typeof window.sign.getBaseHeader !== 'function') {
            return JSON.stringify({success: false, message: 'getBaseHeader is not a function'});
        }
        var result = window.sign.getBaseHeader(e);
        return JSON.stringify({success: true, signatureStr: result.signatureStr || ''});
    } catch (error) {
        return JSON.stringify({success: false, message: error.message});
    }
})()
