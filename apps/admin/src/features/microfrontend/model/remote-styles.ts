/**
 * Чужой CSS живёт в нашем документе — и по умолчанию побеждает наш.
 *
 * Разбор на живом микрофронтенде (Chakra + Mantine) показал две
 * разные утечки, и лечатся они по-разному. Третья, от ремоута на
 * Tailwind v3, — у `layerRemoteStyles` ниже.
 *
 * **Первая — глобальный сброс.** ChakraProvider вкладывает в наш
 * `<head>` десятки тегов `<style data-emotion="css-global">` с
 * правилами на голые элементы:
 *
 *     :where(*)  { border-width: 0 }
 *     button     { background: transparent; padding: 0 }
 *     *,::before { border-color: … }
 *
 * Дело не в весе селекторов, а в СЛОЯХ: Tailwind 4 держит наши утилиты
 * в `@layer utilities`, а нелойерные правила бьют любой слой независимо
 * от специфичности. Поэтому `.border` проигрывает `:where(*)`, и наши
 * кнопки теряют рамку и фон, пока открыт чужой экран.
 *
 * Сужаем их до поддерева ремоута — через `:where(#…)`, а не `#…`.
 * Это принципиально: `:where()` не добавляет веса, и внутри ремоута
 * порядок остаётся ровно тем, на который рассчитывал его автор.
 * Приписать обычный `#id` значило бы поднять сброс над собственными
 * классами Chakra — проверено, ремоут тут же теряет отступы.
 *
 * **Вторая — размер корня.** В их собственном `.css` есть
 * `html { font-size: 12.8px }`, а вся наша вёрстка считает в `rem`.
 * Этот файл приезжает отдельным `<link>` с чужого адреса, и его правила
 * нам не прочитать — CORS. Поэтому не сужаем, а перебиваем своим
 * правилом с `!important`.
 *
 * Ремоут при этом рисуется от нашего корня, то есть крупнее, чем
 * задумал автор. Иначе никак: `rem` считается от корня документа,
 * и «сузить» его до поддерева невозможно в принципе. Выбор между
 * «поедет наша админка» и «чужой экран станет крупнее» сделан
 * в пользу второго.
 *
 * Новым ремоутам трогать `html` и `body` запрещено договором —
 * см. README пакета SDK.
 */

/** Идентификатор узла, в котором живёт чужое дерево. */
const SCOPE_ID = "ucode-remote";
const SCOPE = `:where(#${SCOPE_ID})`;

/** Селекторы, которые целятся в корень документа, переносим НА узел ремоута. */
const ROOT = /(^|[\s>+~,])(?::root|:host|html|body)\b/g;

export function scopeSelector(selector: string): string {
  return selector
    .split(",")
    .map((part) => {
      const one = part.trim();
      if (one.includes(SCOPE_ID)) return one; // уже сузили

      ROOT.lastIndex = 0;
      if (ROOT.test(one)) {
        ROOT.lastIndex = 0;
        return one.replace(ROOT, `$1${SCOPE}`);
      }
      return `${SCOPE} ${one}`;
    })
    .join(", ");
}

/**
 * Только ГЛОБАЛЬНЫЙ сброс ремоута — теги `data-emotion="css-global"`.
 *
 * Стили его компонентов (`data-emotion="css"`) не трогаем никогда, и это
 * не осторожность, а обязательное условие: они адресуются классами
 * (`.css-z93d8g`), нам не мешают, а часть чужого интерфейса живёт
 * ПОРТАЛОМ в `body` — меню строки, всплывашки, диалоги. Сузишь их
 * до узла ремоута — и портал окажется снаружи, без единого стиля.
 * Ровно так и вышло: иконки в меню строки разъехались на пол-экрана.
 */
function foreignGlobalSheets(): CSSStyleSheet[] {
  return [...document.styleSheets].filter(
    (sheet) =>
      sheet.ownerNode instanceof Element &&
      sheet.ownerNode.matches("[data-emotion='css-global']"),
  );
}

/**
 * Правило, которое только ОБЪЯВЛЯЕТ переменные, и ничего не красит.
 *
 * Такие не сужаем — оставляем на всём документе. Имена чужие
 * (`--chakra-*`), нашей вёрстке они безразличны, а ремоуту нужны шире
 * своего узла: часть его интерфейса уходит порталом в `body` — меню
 * строки, всплывашки, диалоги. Загонишь объявления внутрь узла — и
 * в портале `color: var(--chakra-colors-blue-500)` не разрешится:
 * подписи станут чёрными, `gap` схлопнется в `normal`.
 */
function definesOnlyVariables(style: CSSStyleDeclaration): boolean {
  if (style.length === 0) return false;

  for (let i = 0; i < style.length; i += 1) {
    if (!style.item(i).startsWith("--")) return false;
  }
  return true;
}

function scopeForeignRules() {
  for (const sheet of foreignGlobalSheets()) {
    let rules: CSSRule[];
    try {
      rules = [...sheet.cssRules];
    } catch {
      // Чужой origin — правила недоступны. Такой лист гасим целиком
      // при уходе с экрана, см. toggleRemoteStyles.
      continue;
    }

    for (const rule of rules) {
      if (!(rule instanceof CSSStyleRule)) continue;
      if (rule.selectorText.includes(SCOPE_ID)) continue;
      if (definesOnlyVariables(rule.style)) continue;

      try {
        rule.selectorText = scopeSelector(rule.selectorText);
      } catch {
        // Селектор, которого не понимает браузер, — оставляем как есть.
      }
    }
  }
}

/**
 * Вернуть себе размер корня документа. Возвращает функцию отмены.
 *
 * Ставится РАНЬШЕ, чем начинает грузиться ремоут: его `<link>` с
 * `html { font-size: 12.8px }` приезжает вместе с чанками, а это
 * мегабайты и секунды. Успей мы позже — админка на всё это время
 * съезжала бы на 20% и потом прыгала обратно.
 *
 * `medium` — дефолт браузера, то есть настройка ЧЕЛОВЕКА: своего
 * правила на `html` у нас нет вовсе, так что до прихода чужого CSS
 * было ровно оно.
 *
 * `!important` здесь не грубость, а единственный работающий способ.
 * Чужое правило приезжает `<link>`-ом с другого адреса: переписать
 * его нельзя (CORS), слоями не перебить (нелойерное бьёт любой слой),
 * а «мы ниже по документу» ненадёжно — emotion досыпает в `<head>`
 * сотни тегов уже после нас.
 *
 * Счётчик — потому что закрепить корень просят двое: экран (заранее)
 * и сам `RemoteHost` (на случай, если его позовут в обход экрана).
 */
let pinned = 0;
let pin: HTMLStyleElement | null = null;

export function pinRootFontSize(): () => void {
  if (pinned === 0) {
    pin = document.createElement("style");
    pin.dataset.ucode = "root-font-size";
    pin.textContent = "html{font-size:medium!important}";
    document.head.append(pin);
  }
  pinned += 1;

  let released = false;
  return () => {
    if (released) return;
    released = true;
    pinned -= 1;
    if (pinned === 0) {
      pin?.remove();
      pin = null;
    }
  };
}

/**
 * Запереть стили ремоута в его поддереве на всё время, пока он открыт.
 * Возвращает функцию, снимающую всё обратно.
 */
export function containRemoteStyles(container: Element): () => void {
  container.id = SCOPE_ID;
  const unpin = pinRootFontSize();

  scopeForeignRules();

  /*
   * Наблюдатель нужен, а не «сделали один раз»: emotion досыпает стили
   * по мере того, как ремоут рисует новые компоненты, — к моменту
   * монтирования их ещё нет.
   *
   * Проход коалесцируется до одного на кадр: во время монтирования
   * теги сыплются сотнями, и разбирать все листы на каждый — заметная
   * пауза на ровном месте.
   */
  let scheduled = false;
  const observer = new MutationObserver(() => {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(() => {
      scheduled = false;
      scopeForeignRules();
    });
  });
  observer.observe(document.head, { childList: true });

  return () => {
    observer.disconnect();
    unpin();
    container.removeAttribute("id");
  };
}

/**
 * Опустить таблицы стилей с адреса сборки в слой `remote` — на всё время,
 * пока открыт экран. Возвращает функцию, гасящую их обратно.
 *
 * Это **третья утечка**, и она самая широкая. Ремоут на Tailwind v3
 * приносит `<link>` с preflight и утилитами вне слоёв — а значит, поверх
 * всех наших утилит: `a { color: inherit }` бьёт `text-fg-muted`
 * в сайдбаре, `button { padding: 0 }` — отступы кнопок, `*
 * { border-color }` — прозрачную рамку карточки. Классы у v3 и нашего v4
 * те же самые, так что сузить по поддереву мало — порталы ремоута
 * (выпадашки, диалоги) живут в `body` и остались бы без стилей.
 *
 * Слой решает обе стороны: `remote` стоит между нашими `base` и
 * `utilities` (порядок — в app/styles.css). Внутри ремоута его правила
 * по-прежнему бьют наш сброс, снаружи — проигрывают нашим утилитам.
 *
 * `@import … layer()`, а не свой `<style>` с текстом: относительные
 * `url()` шрифтов и картинок браузер разрешит от адреса листа, и CORS
 * не нужен. Оригинальную ссылку не удаляем, а глушим навсегда: плагин
 * `@originjs` помнит вставленные адреса (`seen` в `dynamicLoadingCss`)
 * и второй раз её не создаст — копию при возврате делаем сами.
 *
 * Наблюдатель — потому что ссылка приезжает позже экрана: её вставляет
 * `get()` ремоута, когда тот уже грузится.
 */
export function layerRemoteStyles(entry: string): () => void {
  const origin = originOf(entry);
  if (!origin) return () => {};

  const copies = new Map<string, HTMLStyleElement>();

  const adopt = () => {
    for (const link of document.querySelectorAll<HTMLLinkElement>("link[rel='stylesheet']")) {
      if (!link.href.startsWith(origin) || copies.has(link.href)) continue;

      // `media`, а не `disabled`: ссылку ловим сразу после вставки, листа
      // ещё нет, и Chrome такой `disabled` молча забывает — лист грузится
      // включённым.
      link.media = "not all";
      const copy = document.createElement("style");
      copy.dataset.ucode = "remote-styles";
      copy.textContent = `@import url(${JSON.stringify(link.href)}) layer(remote);`;
      document.head.append(copy);
      copies.set(link.href, copy);
    }
  };

  adopt();
  const observer = new MutationObserver(adopt);
  observer.observe(document.head, { childList: true });

  return () => {
    observer.disconnect();
    for (const copy of copies.values()) copy.remove();
  };
}

function originOf(entry: string): string {
  try {
    return new URL(entry).origin;
  } catch {
    return "";
  }
}
