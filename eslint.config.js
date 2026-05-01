import js from "@eslint/js";
import tseslint from "typescript-eslint";
import globals from "globals";

export default tseslint.config(
    {
        ignores: ["static/js/**/*.min.js", "static/js/*.js", "node_modules/**"]
    },
    js.configs.recommended,
    ...tseslint.configs.strictTypeChecked,
    {
        files: ["static/js/src/**/*.ts"],
        languageOptions: {
            ecmaVersion: 2022,
            sourceType: "module",
            globals: {
                ...globals.browser
            },
            parserOptions: {
                project: "./tsconfig.json"
            }
        },
        rules: {
            // TypeScript strict rules are already enabled via strictTypeChecked
            // Additional rules
            "@typescript-eslint/explicit-function-return-type": "error",
            "@typescript-eslint/no-explicit-any": "error",
            "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
            "@typescript-eslint/strict-boolean-expressions": "off", // Too noisy
            "@typescript-eslint/no-non-null-assertion": "off", // We use these intentionally after null checks

            // Best practices
            "eqeqeq": ["error", "always", { null: "ignore" }],
            "no-eval": "error",
            "no-implied-eval": "error",
            "radix": "error"
        }
    }
);
