(function() {
    try {
        var checks = {
            "demo": typeof demo !== 'undefined',
            "demo.regAction": typeof demo !== 'undefined' && typeof demo.regAction === 'function',
            "window.sign": typeof window.sign !== 'undefined',
            "window.sign.getBaseHeader": typeof window.sign !== 'undefined' && typeof window.sign.getBaseHeader === 'function'
        };

        return JSON.stringify({
            success: Object.keys(checks).every(function(key) {
                return checks[key];
            }),
            checks: checks,
            missingItems: Object.keys(checks).filter(function(key) {
                return !checks[key];
            })
        });
    } catch (error) {
        return JSON.stringify({
            success: false,
            checks: {},
            missingItems: ["reg_action_context"],
            message: error.message
        });
    }
})()
