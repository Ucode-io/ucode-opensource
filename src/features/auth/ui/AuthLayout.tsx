import { useLayoutEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { applyTheme, useUi } from "@/shared/lib/ui-store";
import { ProductPreview } from "./ProductPreview";

/** Общий каркас экранов входа и регистрации: форма слева, бренд справа. */
export function AuthLayout({ children }: { children: ReactNode }) {
  /*
   * Вход всегда светлый, какая бы тема ни стояла у человека.
   *
   * Тёмная тема — настройка рабочего места, а до входа рабочего места
   * ещё нет: экран показывают и тому, у кого своих настроек не будет
   * вовсе (приглашение, восстановление пароля). Панель с окном продукта
   * тоже нарисована как светлая витрина, и в тёмной теме витрина
   * выцветает.
   *
   * useLayoutEffect, а не useEffect: тема снимается ДО отрисовки, иначе
   * при входе в тёмной теме экран моргнёт тёмным на кадр. При уходе
   * возвращается та, что выбрана в настройках.
   */
  useLayoutEffect(() => {
    applyTheme("light");
    return () => applyTheme(useUi.getState().theme);
  }, []);

  return (
    <div className="grid min-h-dvh lg:grid-cols-2">
      <div className="grid place-items-center p-6">{children}</div>

      {/* Панель — карточка с полями, а не половина окна: скруглённый край
          отделяет её от формы сам, без разделительной линии. */}
      <div className="hidden p-3 lg:block">
        <BrandPanel />
      </div>
    </div>
  );
}

/**
 * Панель входа: серое поле, окно продукта и волны внизу.
 *
 * Серый фон, а не заливка брендом: на панели живёт окно продукта, и оно
 * само по себе цветное. Градиент под ним превращал бы белую карточку
 * в наклейку поверх картинки, а текст над ней — в текст, чей контраст
 * зависит от того, куда он попал.
 *
 * Цвет остаётся внизу, волнами, и это единственное место в продукте
 * с градиентом: экран без данных, читать нечего, сканировать нечего.
 * Текст с ними не пересекается — он сверху, на ровном сером.
 */
function BrandPanel() {
  const { t } = useTranslation();

  return (
    /* Самый тёмный нейтральный токен из палитры: панель должна читаться
       полем, на котором лежит окно, а не второй карточкой рядом с формой.
       В тёмной теме он и есть тёмно-серый — картинки-заготовки не нужно. */
    <div aria-hidden className="relative h-full overflow-hidden rounded-3xl bg-surface-active">
      <Waves />

      <div className="relative flex flex-col gap-3 p-10 xl:p-12">
        <p className="text-2xl font-semibold tracking-tight text-fg">{t("auth.tagline")}</p>
        <p className="max-w-sm text-base text-fg-muted">{t("auth.taglineNote")}</p>
      </div>

      {/*
        Окно продукта прижато к нижнему правому углу и уходит за оба
        края панели. Так оно читается как настоящее окно, которое видно
        краем, а не как картинка, поставленная в панель по центру:
        целиком помещённый макет выглядит меньше самого продукта.

        Обрезает его скруглённый край панели — отсюда overflow-hidden
        выше и отрицательные отступы здесь.
      */}
      <div className="absolute -right-16 bottom-24 left-24 top-40 xl:-right-20 xl:left-32">
        <ProductPreview />
      </div>
    </div>
  );
}

/**
 * Волны внизу панели: три слоя одной брендовой шкалы.
 *
 * SVG, а не картинка: цвета берутся из тех же токенов, что и весь
 * продукт, и тянутся по ширине панели, какой бы она ни была
 * (`preserveAspectRatio="none"` — форму волны это не портит, у неё нет
 * деталей, которым важны пропорции).
 *
 * Форма не синусоида, а разлив: гребень у левого края, широкая ложбина
 * посередине и подъём справа. Симметричная волна читается узором,
 * а несимметричная — жидкостью, и слои перекрываются как разлитая вода,
 * а не как гофрокартон.
 *
 * У каждой волны свой наклон градиента, и это не украшательство:
 * три одинаково залитые волны сливаются в одно пятно, а разный наклон
 * даёт каждой свою светлую сторону — и слои читаются слоями.
 *
 *   дальняя   слева направо, наклон не меняется
 *   средняя   сверху вниз
 *   ближняя   снизу вверх
 *
 * Прозрачность нижних слоёв тоже по делу: там, где они накладываются,
 * цвет смешивается, и граница между волнами получается мягкой сама —
 * без обводок и теней.
 */
/* Общие для трёх волн — путь описывается один раз, а не дублируется
   между заливкой и клип-путём шума ниже. */
const WAVE_DEEP_D =
  "M0 64C120 60 208 124 300 212C398 306 520 296 642 282C760 268 830 372 900 400C982 434 1034 340 1122 344C1152 346 1178 354 1200 362V760H0V64Z";
const WAVE_MID_D =
  "M0 300C150 297 300 284 430 258C522 240 562 216 642 220C732 225 800 318 882 380C962 438 1082 440 1200 422V760H0V300Z";
const WAVE_LIGHT_D =
  "M0 470C92 458 172 458 252 526C332 592 352 668 452 694C562 722 702 698 822 640C942 582 1082 560 1200 586V760H0V470Z";

function Waves() {
  return (
    <svg
      /* Высота в пикселях, а не долей панели: волна — рисунок, и высота
         у него своя. От доли она росла бы вместе с экраном, и на большом
         мониторе полоса занимала бы пол-панели. */
      className="absolute inset-x-0 bottom-0 h-[360px] w-full"
      viewBox="0 0 1200 760"
      preserveAspectRatio="none"
      fill="none"
    >
      <defs>
        {/* Наклон задаётся координатами: x — по горизонтали, y — по вертикали. */}
        <linearGradient id="auth-wave-deep" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="var(--color-brand-gradient-from)" />
          <stop offset="100%" stopColor="var(--color-brand-gradient-to)" />
        </linearGradient>
        <linearGradient id="auth-wave-mid" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--color-accent)" />
          <stop offset="100%" stopColor="var(--color-brand-gradient-via)" />
        </linearGradient>
        <linearGradient id="auth-wave-light" x1="0" y1="1" x2="0" y2="0">
          <stop offset="0%" stopColor="var(--color-brand-gradient-to)" />
          <stop offset="100%" stopColor="var(--color-accent)" />
        </linearGradient>
        {/* Блик на гребне: objectBoundingBox (по умолчанию) растягивается
            на bbox своей волны, поэтому «верх» у каждой — свои 35%, а не
            фиксированная линия по вьюбоксу. Белый — не токен, а блик. */}
        <linearGradient id="auth-wave-highlight" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#ffffff" stopOpacity="0.45" />
          <stop offset="35%" stopColor="#ffffff" stopOpacity="0" />
        </linearGradient>
        {/* Зерно поверх волн: чистый feTurbulence, без цвета, поэтому мимо
            правила про токены — это текстура, а не заливка. Область шире
            вьюбокса (x/y -20%), чтобы размытие волн под ним не обрезалось
            краем фильтра. */}
        <filter id="auth-wave-noise" x="-20%" y="-20%" width="140%" height="140%">
          <feTurbulence type="fractalNoise" baseFrequency="0.65" numOctaves="2" seed="7" stitchTiles="stitch" />
          <feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 0.6 0" />
        </filter>
        {/* Тот же контур волн, но как клип: без него шумовой rect красит
            весь вьюбокс, включая прозрачные углы вокруг разлива. */}
        <clipPath id="auth-wave-clip">
          <path d={WAVE_DEEP_D} />
          <path d={WAVE_MID_D} />
          <path d={WAVE_LIGHT_D} />
        </clipPath>
      </defs>

      {/* isolation: держит блендинг шума внутри группы — иначе overlay
          смешивался бы не только с волнами, а со всей панелью под svg. */}
      <g style={{ isolation: "isolate" }}>
        {/* ponytail: блюр временно снят по просьбе — вернуть style={{ filter: "blur(6px)" }} на этот <g>. */}
        <g>
          {/* Дальняя: гребень у левого края, ложбина посередине, подъём справа. */}
          <path d={WAVE_DEEP_D} fill="url(#auth-wave-deep)" opacity="0.55" />
          <path d={WAVE_DEEP_D} fill="url(#auth-wave-highlight)" style={{ mixBlendMode: "soft-light" }} />
          {/* Средняя — самая пологая: её гребень приходится на ложбину дальней. */}
          <path d={WAVE_MID_D} fill="url(#auth-wave-mid)" opacity="0.75" />
          <path d={WAVE_MID_D} fill="url(#auth-wave-highlight)" style={{ mixBlendMode: "soft-light" }} />
          {/* Ближняя — цвет бренда в полную силу: он и должен остаться в глазу. */}
          <path d={WAVE_LIGHT_D} fill="url(#auth-wave-light)" />
          <path d={WAVE_LIGHT_D} fill="url(#auth-wave-highlight)" style={{ mixBlendMode: "soft-light" }} />
        </g>

        {/* Шум сверху, без размытия: делает разлив матовым и шероховатым,
            а не гладким. overlay смешивает его с цветом волны под ним, а не
            просто высветляет — иначе зерно читалось бы белой дымкой.
            clip-path держит его строго в контуре волн. */}
        <rect
          x="0"
          y="0"
          width="1200"
          height="760"
          clipPath="url(#auth-wave-clip)"
          filter="url(#auth-wave-noise)"
          opacity="1"
          style={{ mixBlendMode: "overlay" }}
        />
      </g>
    </svg>
  );
}
