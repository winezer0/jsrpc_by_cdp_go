(function() {
    try {
        return JSON.stringify({
            hasDemo: typeof demo !== 'undefined',
            hasHandlers: typeof demo !== 'undefined' && !!demo.handlers,
            hasSignAction: typeof demo !== 'undefined' && !!demo.handlers && typeof demo.handlers.sign === 'function',
            handlerKeys: typeof demo !== 'undefined' && demo.handlers ? Object.keys(demo.handlers) : []
        });
    } catch (error) {
        return JSON.stringify({
            hasDemo: false,
            hasHandlers: false,
            hasSignAction: false,
            handlerKeys: [],
            message: error.message
        });
    }
})()
