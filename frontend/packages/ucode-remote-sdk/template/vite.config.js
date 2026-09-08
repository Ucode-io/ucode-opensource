import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { ucodeRemote } from "@ucode/remote-sdk/vite";

export default defineConfig({
  plugins: [react(), ucodeRemote({ name: "ucode_remote", page: "./src/Page.jsx" })],
  build: {
    // Админка грузит remoteEntry.js как ES-модуль — сборка обязана
    // им быть, и без предзагрузки чужих чанков в наш документ.
    target: "esnext",
    modulePreload: false,
    outDir: "build",
  },
});
