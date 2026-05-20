(function() {
    try {
        return JSON.stringify({
            success: typeof demo !== 'undefined' && typeof Hlclient !== 'undefined',
            hasDemo: typeof demo !== 'undefined',
            hasHlclient: typeof Hlclient !== 'undefined'
        });
    } catch (error) {
        return JSON.stringify({
            success: false,
            message: error.message
        });
    }
})()
