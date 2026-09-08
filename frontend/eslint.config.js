import js from "@eslint/js";
import tseslint from "typescript-eslint";
import globals from "globals";

/**
 * Границы из docs/PLAN.md. Правило, которое проверяется только чтением,
 * не соблюдается — ни людьми, ни агентами. Поэтому оно здесь.
 *
 * Важно: во flat-config блоки перетирают одноимённые правила целиком,
 * а не дополняют. Поэтому каждый блок собирает свой полный список,
 * а не рассчитывает на предыдущий.
 */

const NO_DEEP_FEATURE = {
  group: ["@/features/*/*"],
  message:
    "Внутренности фичи закрыты. Импортируй из @/features/<имя> (её index.ts). Свои файлы — относительным путём.",
};

const NO_SHARED_DEPENDENCIES = {
  group: ["@/features/*", "@/features/*/*", "@/routes/*", "@/app/*"],
  message: "shared/ не зависит ни от чего. Если нужен features/ — это не shared.",
};

const NO_ROUTES_FROM_FEATURE = {
  group: ["@/routes/*", "@/app/*"],
  message: "Фича не знает о маршрутах. Данные передаёт route, а не наоборот.",
};

const NO_QUERY_HOOKS = {
  name: "@tanstack/react-query",
  importNames: ["useQuery", "useMutation", "useInfiniteQuery", "useSuspenseQuery", "useQueries"],
  message: "Запросы объявляются в features/<имя>/api/. Компонент вызывает готовый хук оттуда.",
};

const restrict = ({ patterns = [], paths = [] }) => ({
  "no-restricted-imports": ["error", { patterns, paths }],
});

export default tseslint.config(
  { ignores: ["dist", "src/routeTree.gen.ts", "src/shared/api/schema.d.ts"] },

  js.configs.recommended,
  ...tseslint.configs.recommended,

  {
    files: ["scripts/**/*.mjs", "*.config.{js,ts}"],
    languageOptions: { globals: globals.node },
  },

  // Всё, что не api/ и не app/: ходить внутрь чужой фичи нельзя, запросы нельзя.
  {
    files: ["src/**/*.{ts,tsx}"],
    languageOptions: { globals: globals.browser },
    rules: restrict({ patterns: [NO_DEEP_FEATURE], paths: [NO_QUERY_HOOKS] }),
  },

  // shared/ не импортирует ниоткуда.
  {
    files: ["src/shared/**/*.{ts,tsx}"],
    rules: restrict({ patterns: [NO_SHARED_DEPENDENCIES], paths: [NO_QUERY_HOOKS] }),
  },

  // features/ не знает о маршрутах и о внутренностях соседей.
  {
    files: ["src/features/**/*.{ts,tsx}"],
    rules: restrict({
      patterns: [NO_ROUTES_FROM_FEATURE, NO_DEEP_FEATURE],
      paths: [NO_QUERY_HOOKS],
    }),
  },

  // Единственное место, где объявляются запросы.
  {
    files: ["src/features/*/api/**/*.{ts,tsx}"],
    rules: restrict({ patterns: [NO_ROUTES_FROM_FEATURE, NO_DEEP_FEATURE] }),
  },

  // app/ собирает приложение — ему queryClient нужен.
  {
    files: ["src/app/**/*.{ts,tsx}"],
    rules: restrict({ patterns: [NO_DEEP_FEATURE] }),
  },

  {
    files: ["**/*.test.{ts,tsx}"],
    rules: { "@typescript-eslint/no-explicit-any": "off" },
  },
);
