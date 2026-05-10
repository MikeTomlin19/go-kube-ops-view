const js = require('@eslint/js')

module.exports = [
    {
        ignores: ['src/vendor/*.js'],
    },
    js.configs.recommended,
    {
        files: ['src/**/*.js'],
        languageOptions: {
            ecmaVersion: 2022,
            sourceType: 'module',
            globals: {
                document: 'readonly',
                window: 'readonly',
                localStorage: 'readonly',
                location: 'readonly',
                addEventListener: 'readonly',
                XMLHttpRequest: 'readonly',
                EventSource: 'readonly',
                console: 'readonly',
                setTimeout: 'readonly',
                setInterval: 'readonly',
                clearTimeout: 'readonly',
                clearInterval: 'readonly',
                Promise: 'readonly',
                process: 'readonly',
                module: 'readonly',
                require: 'readonly',
                __dirname: 'readonly',
            },
        },
        rules: {
            indent: ['error', 4],
            'linebreak-style': ['error', 'unix'],
            quotes: ['error', 'single'],
            'prefer-const': ['error'],
            'no-redeclare': ['error'],
            'no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
            semi: ['error', 'never'],
        },
    },
]
