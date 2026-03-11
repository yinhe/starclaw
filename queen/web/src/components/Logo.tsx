export function LogoIcon({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 19c-2 0-4-1-5-3s-1-4 0-6c1-1.5 3-3 5-3s4 1.5 5 3c1 2 1 4 0 6s-3 3-5 3z"/>
      <path d="M9 10c-2-2-4-3-6-2"/>
      <path d="M15 10c2-2 4-3 6-2"/>
      <path d="M8 7c-1-2-1-4 0-5"/>
      <path d="M16 7c1-2 1-4 0-5"/>
      <circle cx="10" cy="11" r="0.8" fill="white"/>
      <circle cx="14" cy="11" r="0.8" fill="white"/>
      <path d="M10 16c0.5 0.5 1.5 1 2 1s1.5-0.5 2-1"/>
    </svg>
  );
}

export function LogoMark({ className = 'w-8 h-8' }: { className?: string }) {
  return (
    <div className={`${className} rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center`}>
      <LogoIcon />
    </div>
  );
}

export function LogoFull() {
  return (
    <div className="flex items-center gap-2.5">
      <LogoMark />
      <span className="text-xl font-bold text-gray-900">StarClaw</span>
    </div>
  );
}
