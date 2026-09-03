package registry

// entries11 создаёт полный набор независимых записей DSL 1.1.
func entries11() []Entry {
	const both = FormInline | FormBlock
	return []Entry{
		{Name: "dsl-version", Forms: FormInline, Arguments: ArgsVersion, Universal: HandlerTokens, Special: HandlerVersion},
		{Name: "task", Forms: FormInline, Arguments: ArgsID, Universal: HandlerTokens, Context: ContextTaskStart},
		{Name: "endtask", Forms: FormInline, Context: ContextTaskEnd},
		{Name: "header", Forms: FormInline, Arguments: ArgsTitle, Universal: HandlerFreeText, Context: ContextTaskBoundary},
		{Name: "newpage", Forms: FormInline, Context: ContextStepBoundary},
		{Name: "step", Forms: FormInline, Arguments: ArgsTitle, Universal: HandlerFreeText, Context: ContextStepStart},
		{Name: "editor", Forms: both, Arguments: ArgsContent, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "speaking", Forms: FormInline, Context: ContextSpeaking},
		{Name: "media", Forms: FormInline, Arguments: ArgsMediaTokens, Universal: HandlerTokens, Special: HandlerMedia},
		{Name: "example", Forms: both, Arguments: ArgsContent, Universal: HandlerFreeText, Body: BodyOpaqueShaped, BodyHandler: HandlerExample},
		{Name: "wordlist", Forms: both, Arguments: ArgsContent, Universal: HandlerFreeText, Body: BodyOpaqueShaped, BodyHandler: HandlerWordlist},
		{Name: "table", Forms: FormBlock, Arguments: ArgsOptionalTitle, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "script", Forms: FormBlock, Arguments: ArgsOptionalTitle, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "text", Forms: FormBlock, Arguments: ArgsOptionalTitle, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody, PreserveEmpty: true},
		{Name: "key", Forms: FormBlock, Arguments: ArgsOptionalTitle, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "instr", Forms: both, Arguments: ArgsContent, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "note", Forms: both, Arguments: ArgsContent, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "alt", Forms: both, Arguments: ArgsContent, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody},
		{Name: "question", Forms: FormInline, Arguments: ArgsContent, Universal: HandlerFreeText},
		{Name: "multifill", Forms: both, Arguments: ArgsOptionalInstruction, Universal: HandlerFreeText, Body: BodyOpaqueRaw, BodyHandler: HandlerRawBody, PreserveEmpty: true},
		{Name: "choice", Forms: FormBlock, Arguments: ArgsOptionalInstruction, Universal: HandlerFreeText, Body: BodyOpaqueShaped, BodyHandler: HandlerItems},
		{Name: "multichoice", Forms: FormBlock, Arguments: ArgsOptionalInstruction, Universal: HandlerFreeText, Body: BodyOpaqueShaped, BodyHandler: HandlerItems},
		{Name: "matching", Forms: FormBlock, Arguments: ArgsOptionalInstruction, Universal: HandlerFreeText, Body: BodyOpaqueShaped, BodyHandler: HandlerMatching},
		{Name: "ordering", Forms: FormBlock, Arguments: ArgsOptionalInstruction, Universal: HandlerFreeText, Body: BodyOpaqueShaped, BodyHandler: HandlerOrdering},
		{Name: "variants", Forms: FormBlock, Body: BodyStructural, BodyHandler: HandlerStructural, Context: ContextVariants},
		{Name: "variant", Forms: FormInline, Arguments: ArgsName, Universal: HandlerFreeText, Body: BodyLogical, Context: ContextVariant},
	}
}
