export function TypingIndicator() {
	return (
		<div className="flex items-center gap-1 py-1">
			<div className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:-0.3s]" />
			<div className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:-0.15s]" />
			<div className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground" />
		</div>
	);
}
