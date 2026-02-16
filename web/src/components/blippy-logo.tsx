export function BlippyLogo({ className }: { className?: string }) {
	return (
		<svg
			viewBox="25 10 155 160"
			xmlns="http://www.w3.org/2000/svg"
			className={className}
			role="img"
			aria-label="Blippy logo"
		>
			<path
				d="M 55,155 C 40,130 32,100 38,70 C 44,40 68,20 100,18 C 132,16 160,38 166,70 C 172,102 162,140 145,160"
				fill="none"
				stroke="currentColor"
				strokeWidth="4"
				strokeLinecap="round"
			/>
			<path
				d="M 58,80 C 64,62 88,60 92,80"
				fill="none"
				stroke="currentColor"
				strokeWidth="4.5"
				strokeLinecap="round"
			/>
			<circle cx="132" cy="74" r="8" fill="currentColor" />
			<path
				d="M 105,85 L 95,120 L 110,115"
				fill="none"
				stroke="currentColor"
				strokeWidth="3"
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<path
				d="M 70,135 C 82,152 118,155 135,140"
				fill="none"
				stroke="currentColor"
				strokeWidth="4"
				strokeLinecap="round"
			/>
			<circle cx="145" cy="120" r="4" fill="currentColor" />
		</svg>
	);
}
