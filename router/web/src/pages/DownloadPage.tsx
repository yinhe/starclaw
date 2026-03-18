import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Monitor, Terminal, Apple, Download, ExternalLink, Zap } from 'lucide-react';

const STARAI_BASE = 'https://star-ai.net/downloads';
const NYDUS_BASE = 'https://nydus.starclaw.net/spore/releases';

const V = 'v2026.0318.0738';

const PACKAGES = [
  {
    platform: 'Windows',
    icon: Monitor,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}.exe`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}.exe`,
    size: '~26 MB',
    arch: 'x86_64',
    filename: `StarClaw-Setup-${V}.exe`,
    note: '下载后双击运行，图形界面引导安装',
  },
  {
    platform: 'Linux',
    icon: Terminal,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}-linux-amd64.tar.gz`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}-linux-amd64.tar.gz`,
    size: '~19 MB',
    arch: 'x86_64',
    filename: `StarClaw-Setup-${V}-linux-amd64.tar.gz`,
    note: '下载解压后运行 ./StarClaw-Setup',
  },
  {
    platform: 'macOS (Apple 芯片)',
    icon: Apple,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}-darwin-arm64.dmg`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}-darwin-arm64.dmg`,
    size: '~26 MB',
    arch: 'Apple Silicon (M1/M2/M3/M4)',
    filename: `StarClaw-Setup-${V}-darwin-arm64.dmg`,
    note: '下载后打开 DMG，双击 Install StarClaw',
  },
  {
    platform: 'macOS (Intel)',
    icon: Apple,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}-darwin-amd64.dmg`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}-darwin-amd64.dmg`,
    size: '~27 MB',
    arch: 'Intel x86_64',
    filename: `StarClaw-Setup-${V}-darwin-amd64.dmg`,
    note: '下载后打开 DMG，双击 Install StarClaw',
  },
];

export default function DownloadPage() {
  const [version, setVersion] = useState<string>('');

  useEffect(() => {
    fetch('https://nydus.starclaw.net/releases/latest')
      .then(r => r.json())
      .then(d => {
        const v = d.tag_name || '';
        setVersion(v.startsWith('v') ? v : 'v' + v);
      })
      .catch(() => {});
  }, []);

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      <nav className="border-b border-gray-800/50 backdrop-blur-sm sticky top-0 z-50 bg-gray-950/80">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 bg-gradient-to-br from-amber-400 to-orange-500 rounded-lg flex items-center justify-center">
              <Zap className="w-4.5 h-4.5 text-white" />
            </div>
            <span className="text-lg font-bold tracking-tight">Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span></span>
          </div>
          <div className="flex items-center gap-3">
            <Link to="/" className="text-sm text-gray-400 hover:text-white px-3 py-2 transition-colors">首页</Link>
            <Link to="/download" className="text-sm text-gray-300 hover:text-white px-3 py-2 transition-colors">下载</Link>
            <Link to="/login" className="text-sm text-gray-300 hover:text-white px-3 py-2 transition-colors">登录</Link>
            <Link to="/register" className="text-sm bg-amber-500 hover:bg-amber-400 text-gray-900 font-medium px-4 py-2 rounded-lg transition-colors">
              开始使用
            </Link>
          </div>
        </div>
      </nav>

      <div className="max-w-6xl mx-auto px-6 py-16 space-y-8">
        <div>
          <h1 className="text-2xl font-bold text-white">
            下载客户端
            {version && <span className="ml-3 text-base font-medium text-amber-400">{version}</span>}
          </h1>
          <p className="text-gray-400 text-sm mt-1">安装 StarClaw 客户端，在本地运行 AI Agent</p>
        </div>

        <div className="grid gap-4">
          {PACKAGES.map((pkg) => {
            const Icon = pkg.icon;
            return (
              <div key={pkg.platform} className="bg-gray-900 border border-gray-800 rounded-xl p-5 hover:border-amber-500/30 transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-xl bg-gray-800 flex items-center justify-center">
                      <Icon className="w-6 h-6 text-amber-400" />
                    </div>
                    <div>
                      <div className="font-semibold text-white text-lg">{pkg.platform}</div>
                      <div className="text-xs text-gray-500 mt-0.5">{pkg.arch} · {pkg.size} · {pkg.filename}</div>
                      <div className="text-xs text-gray-400 mt-1">{pkg.note}</div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <a
                      href={pkg.mirrorUrl}
                      className="flex items-center gap-1.5 px-4 py-2 bg-amber-500 hover:bg-amber-400 text-black font-medium rounded-lg text-sm transition-colors"
                    >
                      <Download className="w-4 h-4" /> 下载
                    </a>
                    <a
                      href={pkg.fallbackUrl}
                      className="flex items-center gap-1.5 px-3 py-2 border border-gray-700 hover:border-gray-600 text-gray-400 hover:text-gray-200 rounded-lg text-sm transition-colors"
                      title="海外备用下载"
                    >
                      <ExternalLink className="w-3.5 h-3.5" /> 备用
                    </a>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h2 className="text-lg font-semibold text-white mb-3">一键安装脚本</h2>
          <div className="space-y-3">
            <div>
              <div className="text-xs text-gray-400 mb-1">Linux / macOS</div>
              <code className="block bg-gray-950 border border-gray-800 rounded-lg px-4 py-2.5 text-sm text-amber-400 font-mono select-all">
                curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh
              </code>
            </div>
            <div>
              <div className="text-xs text-gray-400 mb-1">Windows (PowerShell)</div>
              <code className="block bg-gray-950 border border-gray-800 rounded-lg px-4 py-2.5 text-sm text-amber-400 font-mono select-all">
                irm https://nydus.starclaw.net/spore/install.ps1 | iex
              </code>
            </div>
          </div>
        </div>

        <div className="text-xs text-gray-500 space-y-1">
          <p>• 默认下载地址为国内加速镜像，"备用"为海外节点</p>
          <p>• 安装后在「模型管理」页面配置 API Key 即可开始使用</p>
          <p>• 卸载方法：运行 <code className="text-gray-400 bg-gray-800 px-1.5 py-0.5 rounded">StarClaw-Setup --uninstall</code></p>
        </div>
      </div>

      <footer className="border-t border-gray-800/50">
        <div className="max-w-6xl mx-auto px-6 py-10">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-6 mb-8">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 bg-gradient-to-br from-amber-400 to-orange-500 rounded-lg flex items-center justify-center">
                <Zap className="w-3.5 h-3.5 text-white" />
              </div>
              <span className="font-bold tracking-tight">Star<span className="text-amber-400">AI</span></span>
              <span className="text-gray-600 text-xs ml-2">AI 算力平台</span>
            </div>
            <div className="flex items-center gap-6 text-sm text-gray-400">
              <Link to="/" className="hover:text-gray-200 transition-colors">首页</Link>
              <span className="text-gray-700">|</span>
              <Link to="/terms" className="hover:text-gray-200 transition-colors">服务条款</Link>
              <span className="text-gray-700">|</span>
              <Link to="/privacy" className="hover:text-gray-200 transition-colors">隐私政策</Link>
              <span className="text-gray-700">|</span>
              <a href="mailto:service@star-ai.net" className="hover:text-gray-200 transition-colors">联系我们</a>
            </div>
          </div>
          <div className="border-t border-gray-800/50 pt-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-gray-600">
            <div>
              © 2026 [ STARAI ] - 浙江银河天启科技有限公司 版权所有
            </div>
            <div className="flex items-center gap-4">
              <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener" className="hover:text-gray-400 transition-colors">
                浙ICP备2020032632号-5
              </a>
              <a href="https://github.com/yinhe/starclaw" target="_blank" className="hover:text-gray-400 transition-colors">GitHub</a>
              <a href="https://starclaw.me" target="_blank" className="hover:text-gray-400 transition-colors">StarClaw</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
