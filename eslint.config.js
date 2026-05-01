import js from "@eslint/js";
import globals from "globals";

export default [
    {
        ignores: ["static/js/**/*.min.js"]
    },
    js.configs.recommended,
    {
        files: ["static/js/**/*.js"],
        languageOptions: {
            ecmaVersion: 2022,
            sourceType: "script",
            globals: {
                ...globals.browser,
                // Bootstrap
                bootstrap: "readonly",
                // Fuse.js
                Fuse: "readonly",
                // App globals defined in base.html
                showError: "readonly"
            }
        },
        rules: {
            // Catch common errors
            "no-undef": "error",
            "no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
            "no-redeclare": "error",
            "no-dupe-keys": "error",
            "no-duplicate-case": "error",
            "no-empty": "warn",
            "no-unreachable": "error",
            "no-constant-condition": "warn",
            "use-isnan": "error",
            "valid-typeof": "error",

            // Best practices
            "eqeqeq": ["error", "always", { null: "ignore" }],
            "no-eval": "error",
            "no-implied-eval": "error",
            "no-new-func": "error",
            "no-return-assign": "error",
            "no-self-assign": "error",
            "no-self-compare": "error",
            "no-sequences": "error",
            "no-throw-literal": "error",
            "no-unmodified-loop-condition": "error",
            "no-useless-concat": "warn",
            "no-useless-return": "warn",
            "radix": "error",

            // Style (minimal, just catching likely bugs)
            "no-mixed-spaces-and-tabs": "error"
        }
    }
];
