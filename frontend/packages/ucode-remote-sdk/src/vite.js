import federation from "@originjs/vite-plugin-federation";

/**
 * Пресет сборки микрофронтенда ucode.
 *
 * Всё, что здесь зашито, зашито не для удобства, а потому что каждая
 * строчка уже ломалась в бою — см. docs/adr/0005 и 0009 админки.
 *
 * @param {{ name: string, page: string }} options
 *   name — имя ремоута, page — путь к компоненту страницы.
 */
export function ucodeRemote({ name, page }) {
  return federation({
    name,
    filename: "remoteEntry.js",
    exposes: {
      /*
       * Ровно `./Page`. У прежних шаблонов имя разъехалось (`./App`),
       * и хост вынужден спрашивать оба — но это касается только
       * старого поколения.
       */
      "./Page": page,
      // Самоописание: по нему хост узнаёт, что ремоут новый.
      "./ucode": "@ucode/remote-sdk/meta",
    },
    shared: {
      /*
       * `requiredVersion` — не опция, а условие работы. Без него
       * потребитель берёт ПЕРВУЮ версию из общей области хоста
       * (@originjs/dist/index.js:956), а первой там лежит React 18 —
       * для ремоутов прежнего поколения. Ремоут договора 1 получил бы
       * React 18 и упал на элементах чужого поколения.
       */
      react: { requiredVersion: "^19" },
      "react-dom": { requiredVersion: "^19" },
      /*
       * jsx-runtime общий наравне с react: JSX собирается ИМЕННО им,
       * а элемент, созданный чужим рантаймом, React 19 хоста отвергает
       * ошибкой #525.
       */
      "react/jsx-runtime": { requiredVersion: "^19" },
    },
  });
}
