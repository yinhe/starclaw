import { LogoMark } from './Logo';

export function Footer() {
  return (
    <footer className="border-t border-gray-200 py-12">
      <div className="max-w-7xl mx-auto px-6">
        <div className="flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2.5">
            <LogoMark className="w-7 h-7" />
            <span className="font-bold text-gray-900">StarClaw</span>
          </div>
          <div className="flex items-center gap-6 text-sm text-gray-500">
            <a href="https://github.com/yinhe/starclaw" target="_blank" rel="noreferrer" className="hover:text-gray-900 transition">GitHub</a>
            <a href="https://app.starclaw.me" className="hover:text-gray-900 transition">在线体验</a>
            <a href="https://github.com/yinhe/starclaw/issues" target="_blank" rel="noreferrer" className="hover:text-gray-900 transition">反馈问题</a>
          </div>
          <p className="text-sm text-gray-400">&copy; 2025 StarClaw. MIT License.</p>
        </div>
      </div>
    </footer>
  );
}
