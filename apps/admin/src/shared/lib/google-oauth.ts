/**
 * Вход через Google — необязательная возможность.
 *
 * У самостоятельной установки приложения в Google Cloud может не быть вовсе.
 * Библиотека @react-oauth/google этого не прощает с двух сторон:
 * GoogleOAuthProvider бросает исключение без clientId, а её хуки — если
 * провайдера нет над ними. Поэтому и провайдер, и кнопка включаются по одному
 * и тому же признаку, объявленному здесь, чтобы они не могли разойтись.
 */
export const googleClientId = import.meta.env.VITE_GOOGLE_CLIENT_ID ?? "";

export const isGoogleLoginEnabled = googleClientId.trim() !== "";
