#include "clang_bridge.h"

#include <algorithm>
#include <atomic>
#include <string>
#include <utility>
#include <vector>

#include <clang-c/Index.h>

namespace
{

std::atomic<unsigned long long> parse_count{0};

struct hard_namespace
{
	std::string name;
	bool is_inline = false;
};

struct hard_include
{
	std::string source;
	std::string target;
	std::string spelling;
	bool is_system = false;
};

struct hard_declaration
{
	std::string file;
	std::string name;
	std::string kind;
	bool is_definition = false;
	bool is_specialization = false;
	unsigned offset = 0;
	std::vector<hard_namespace> namespaces;
	std::vector<std::string> template_parameters;
	std::string identity;
	std::string enum_base;
	std::string constraint;
	std::string unsupported;
	std::vector<std::string> requirements;
};

struct hard_function
{
	std::string file;
	std::string name;
	bool is_definition = false;
	bool is_global = false;
};

struct hard_diagnostic
{
	unsigned severity = 0;
	std::string text;
	std::string category;
	std::string file;
	unsigned line = 0;
	unsigned column = 0;
};

std::string to_string(CXString value)
{
	std::string result;
	if (const char* data = clang_getCString(value))
	{
		result = data;
	}
	clang_disposeString(value);
	return result;
}

std::string file_name(CXFile file)
{
	if (!file)
	{
		return {};
	}
	return to_string(clang_getFileName(file));
}

bool declaration_namespaces(CXCursor cursor, std::vector<hard_namespace>& namespaces)
{
	CXCursor parent = clang_getCursorSemanticParent(cursor);
	while (!clang_Cursor_isNull(parent))
	{
		switch (clang_getCursorKind(parent))
		{
			case CXCursor_TranslationUnit:
				std::reverse(namespaces.begin(), namespaces.end());
				return true;
			case CXCursor_Namespace:
			{
				std::string name = to_string(clang_getCursorSpelling(parent));
				if (name.empty())
				{
					return false;
				}
				namespaces.push_back({std::move(name), clang_Cursor_isInlineNamespace(parent) != 0});
				break;
			}
			case CXCursor_LinkageSpec:
				break;
			default:
				return false;
		}
		parent = clang_getCursorSemanticParent(parent);
	}
	return false;
}

bool is_direct_declaration(CXCursor cursor)
{
	CXCursor parent = clang_getCursorLexicalParent(cursor);
	while (!clang_Cursor_isNull(parent))
	{
		switch (clang_getCursorKind(parent))
		{
			case CXCursor_TranslationUnit:
				return true;
			case CXCursor_Namespace:
			case CXCursor_LinkageSpec:
				parent = clang_getCursorLexicalParent(parent);
				break;
			default:
				return false;
		}
	}
	return false;
}

bool is_global_function(CXCursor cursor)
{
	CXCursor parent = clang_getCursorSemanticParent(cursor);
	while (!clang_Cursor_isNull(parent))
	{
		switch (clang_getCursorKind(parent))
		{
			case CXCursor_TranslationUnit:
				return true;
			case CXCursor_LinkageSpec:
				parent = clang_getCursorSemanticParent(parent);
				break;
			default:
				return false;
		}
	}
	return false;
}

struct signature_token
{
	std::string text;
	CXTokenKind kind;
	CXCursor cursor;
};

std::vector<signature_token> signature_tokens(CXTranslationUnit unit, CXSourceRange range)
{
	CXToken* tokens = nullptr;
	unsigned token_count = 0;
	clang_tokenize(unit, range, &tokens, &token_count);
	std::vector<CXCursor> cursors(token_count);
	clang_annotateTokens(unit, tokens, token_count, cursors.data());
	std::vector<signature_token> result;
	for (unsigned index = 0; index < token_count; ++index)
	{
		if (clang_getTokenKind(tokens[index]) == CXToken_Comment)
			continue;
		CXCursor cursor = clang_getCursor(unit, clang_getTokenLocation(unit, tokens[index]));
		if (clang_getCursorKind(cursor) != CXCursor_MacroExpansion)
			cursor = cursors[index];
		result.push_back({to_string(clang_getTokenSpelling(unit, tokens[index])), clang_getTokenKind(tokens[index]), cursor});
	}
	clang_disposeTokens(unit, tokens, token_count);
	return result;
}

bool template_parameter_kind(CXCursorKind kind)
{
	return kind == CXCursor_TemplateTypeParameter || kind == CXCursor_NonTypeTemplateParameter ||
	       kind == CXCursor_TemplateTemplateParameter;
}

// References come from libclang, not identifier guessing. Only opaque enums
// can supply an external value-parameter type without needing a definition.
// Aliases, concepts, values, and other header-local context are not moved.
void inspect_signature_token(const signature_token& token, hard_declaration& value)
{
	if (token.kind != CXToken_Identifier)
		return;
	CXCursorKind kind = clang_getCursorKind(token.cursor);
	if (kind == CXCursor_MacroExpansion)
	{
		value.unsupported = "macro in template signature: " + token.text;
		return;
	}
	CXCursor reference = clang_getCursorReferenced(token.cursor);
	CXCursorKind reference_kind = clang_getCursorKind(reference);
	if (template_parameter_kind(kind) || template_parameter_kind(reference_kind) ||
	    reference_kind == CXCursor_Namespace)
		return;
	if (reference_kind == CXCursor_EnumDecl)
	{
		value.requirements.push_back(to_string(clang_getCursorUSR(reference)));
		return;
	}
	value.unsupported = "template signature needs header context: " + token.text;
}

std::string render_signature(const std::vector<signature_token>& tokens, hard_declaration& value)
{
	std::string result;
	for (const auto& token : tokens)
	{
		inspect_signature_token(token, value);
		if (!result.empty())
			result += ' ';
		result += token.text;
	}
	return result;
}

// A value parameter's typedef can be written as its canonical builtin type
// (e.g. size_t -> unsigned long) without importing the typedef's header. Do
// not do this in constraints: their original token identity must be retained.
bool canonicalize_parameter_types(std::vector<signature_token>& tokens)
{
	bool changed = false;
	for (size_t index = 0; index < tokens.size(); ++index)
	{
		if (clang_getCursorKind(tokens[index].cursor) != CXCursor_TypeRef)
			continue;
		CXCursor reference = clang_getCursorReferenced(tokens[index].cursor);
		CXCursorKind kind = clang_getCursorKind(reference);
		if (kind != CXCursor_TypedefDecl && kind != CXCursor_TypeAliasDecl)
			continue;
		CXType type = clang_getCanonicalType(clang_getTypedefDeclUnderlyingType(reference));
		if (type.kind < CXType_Bool || type.kind > CXType_LongDouble)
			continue;
		size_t first = index;
		while (first > 0 && tokens[first - 1].text == "::")
		{
			--first;
			if (first > 0 && tokens[first - 1].kind == CXToken_Identifier)
				--first;
		}
		tokens[index].text = to_string(clang_getTypeSpelling(type));
		tokens[index].kind = CXToken_Keyword;
		changed = true;
		tokens.erase(tokens.begin() + first, tokens.begin() + index);
		index = first;
	}
	return changed;
}

struct template_context
{
	CXTranslationUnit unit;
	hard_declaration* value;
	CXSourceLocation last_parameter;
};

CXChildVisitResult inspect_expanded_parameter(CXCursor cursor, CXCursor, CXClientData data)
{
	auto& value = *static_cast<hard_declaration*>(data);
	CXCursorKind kind = clang_getCursorKind(cursor);
	if (clang_isReference(kind) || kind == CXCursor_DeclRefExpr)
	{
		inspect_signature_token({to_string(clang_getCursorSpelling(cursor)), CXToken_Identifier, cursor}, value);
	}
	return CXChildVisit_Recurse;
}

CXChildVisitResult collect_template_parameter(CXCursor cursor, CXCursor, CXClientData data)
{
	auto& context = *static_cast<template_context*>(data);
	if (!template_parameter_kind(clang_getCursorKind(cursor)))
		return CXChildVisit_Continue;
	CXSourceRange extent = clang_getCursorExtent(cursor);
	context.last_parameter = clang_getRangeEnd(extent);
	// Print the semantic declaration: raw source ranges can be empty or still
	// contain macro names even though the complete parameter is in the AST.
	std::string original = to_string(clang_getCursorPrettyPrinted(cursor, nullptr));
	if (original.empty())
	{
		context.value->unsupported = "template parameter printing unavailable";
		return CXChildVisit_Continue;
	}
	auto tokens = signature_tokens(context.unit, extent);
	std::string name = to_string(clang_getCursorSpelling(cursor));
	for (auto& token : tokens)
	{
		if (!name.empty() && token.text == name)
			token.cursor = cursor;
	}
	int angles = 0, parentheses = 0, brackets = 0, braces = 0;
	for (size_t index = 0; index < tokens.size(); ++index)
	{
		const auto& text = tokens[index].text;
		if (text == "=" && angles == 0 && parentheses == 0 && brackets == 0 && braces == 0)
		{
			tokens.resize(index);
			break;
		}
		if (text == "(")
			++parentheses;
		if (text == ")")
			--parentheses;
		if (text == "[")
			++brackets;
		if (text == "]")
			--brackets;
		if (text == "{")
			++braces;
		if (text == "}")
			--braces;
		if (parentheses == 0 && brackets == 0 && braces == 0)
		{
			if (text == "<")
				++angles;
			if (text == ">")
				--angles;
			if (text == ">>")
				angles -= 2;
		}
	}
	if (tokens.empty() || std::any_of(tokens.begin(), tokens.end(), [](const signature_token& token)
	    { return clang_getCursorKind(token.cursor) == CXCursor_MacroExpansion; }))
	{
		// Lexical annotations cannot describe an expanded macro's dependencies.
		// Inspect its AST references before using the printed parameter instead.
		clang_visitChildren(cursor, inspect_expanded_parameter, context.value);
		context.value->template_parameters.push_back(std::move(original));
		return CXChildVisit_Continue;
	}
	if (clang_getCursorKind(cursor) == CXCursor_NonTypeTemplateParameter)
	{
		bool canonical = canonicalize_parameter_types(tokens);
		// Keep AST printing unless a builtin typedef needs canonical spelling.
		std::string prefix = render_signature(tokens, *context.value);
		context.value->template_parameters.push_back(canonical ? std::move(prefix) : std::move(original));
	}
	else
	{
		render_signature(tokens, *context.value);
		context.value->template_parameters.push_back(std::move(original));
	}
	return CXChildVisit_Continue;
}

void template_metadata(CXTranslationUnit unit, CXCursor cursor, hard_declaration& value)
{
	template_context context{unit, &value, clang_getNullLocation()};
	clang_visitChildren(cursor, collect_template_parameter, &context);
	if (value.template_parameters.empty())
	{
		value.unsupported = "template parameters unavailable";
		return;
	}
	// The cursor location is the class name. This range includes the closing
	// template bracket, optional requires-clause, and the class-key only.
	auto suffix = signature_tokens(unit, clang_getRange(context.last_parameter, clang_getCursorLocation(cursor)));
	if (!suffix.empty() && suffix.back().text == value.name)
		suffix.pop_back();
	while (!suffix.empty() && suffix.front().text == ">")
		suffix.erase(suffix.begin());
	if (suffix.empty() || (suffix.back().text != "class" && suffix.back().text != "struct"))
	{
		value.unsupported = "template class-key or attributes unavailable";
		return;
	}
	suffix.pop_back();
	if (!suffix.empty())
	{
		if (suffix.front().text != "requires")
			value.unsupported = "unsupported template prefix";
		else
			value.constraint = render_signature(suffix, value);
	}
}

void enum_metadata(CXTranslationUnit unit, CXCursor cursor, hard_declaration& value)
{
	bool scoped = clang_EnumDecl_isScoped(cursor) != 0;
	value.kind = scoped ? "enum class" : "enum";
	bool fixed = false;
	// Stop at the opening brace: colons in enumerator expressions are unrelated.
	for (const auto& token : signature_tokens(unit, clang_getCursorExtent(cursor)))
	{
		if (token.text == "{")
			break;
		if (token.text == ":")
			fixed = true;
	}
	if (!scoped && !fixed)
	{
		value.unsupported = "unscoped enum has no explicit underlying type";
		return;
	}
	CXType base = clang_getCanonicalType(clang_getEnumDeclIntegerType(cursor));
	if (base.kind == CXType_Invalid)
		value.unsupported = "enum underlying type unavailable";
	else
		value.enum_base = to_string(clang_getTypeSpelling(base));
}

} // namespace

struct hard_clang_analysis
{
	CXIndex index = nullptr;
	CXTranslationUnit unit = nullptr;
	std::string error;
	std::vector<hard_include> includes;
	std::vector<hard_declaration> declarations;
	std::vector<hard_function> functions;
	std::vector<hard_diagnostic> diagnostics;
};

namespace
{

CXChildVisitResult visit_cursor(CXCursor cursor, CXCursor, CXClientData client_data)
{
	auto* analysis = static_cast<hard_clang_analysis*>(client_data);
	CXCursorKind cursor_kind = clang_getCursorKind(cursor);
	if (cursor_kind == CXCursor_InclusionDirective)
	{
		CXFile source_file = nullptr;
		clang_getExpansionLocation(
		        clang_getCursorLocation(cursor),
		        &source_file,
		        nullptr,
		        nullptr,
		        nullptr);
		CXFile target_file = clang_getIncludedFile(cursor);
		bool is_system = false;
		if (target_file)
		{
			CXSourceLocation target_location = clang_getLocation(analysis->unit, target_file, 1, 1);
			is_system = clang_Location_isInSystemHeader(target_location) != 0;
		}
		analysis->includes.push_back({
		        file_name(source_file),
		        file_name(target_file),
		        to_string(clang_getCursorSpelling(cursor)),
		        is_system,
		});
		return CXChildVisit_Recurse;
	}

	bool declaration = cursor_kind == CXCursor_ClassDecl ||
	                   cursor_kind == CXCursor_EnumDecl ||
	                   cursor_kind == CXCursor_StructDecl ||
	                   cursor_kind == CXCursor_ClassTemplate ||
	                   cursor_kind == CXCursor_ClassTemplatePartialSpecialization;
	if (declaration)
	{
		if (clang_Location_isInSystemHeader(clang_getCursorLocation(cursor))) return CXChildVisit_Continue;
		// libclang spells anonymous typedef tags using the alias. Declaring that
		// spelling as a tag would conflict with the original C typedef. Such
		// cursors point at the tag keyword, even when isAnonymous returns false.
		if (clang_Cursor_isAnonymous(cursor)) return CXChildVisit_Recurse;
		CXToken* name_token = clang_getToken(analysis->unit, clang_getCursorLocation(cursor));
		if (name_token)
		{
			std::string spelling = to_string(clang_getTokenSpelling(analysis->unit, *name_token));
			clang_disposeTokens(analysis->unit, name_token, 1);
			if (spelling == "struct" || spelling == "class" || spelling == "enum")
				return CXChildVisit_Recurse;
		}
		hard_declaration value;
		if (!is_direct_declaration(cursor) ||
		    !declaration_namespaces(cursor, value.namespaces))
		{
			return CXChildVisit_Recurse;
		}
		CXFile file = nullptr;
		clang_getExpansionLocation(
		        clang_getCursorLocation(cursor),
		        &file,
		        nullptr,
		        nullptr,
		        &value.offset);
		value.file = file_name(file);
		value.name = to_string(clang_getCursorSpelling(cursor));
		value.identity = to_string(clang_getCursorUSR(cursor));
		CXCursorKind template_kind = cursor_kind;
		if (cursor_kind == CXCursor_ClassTemplate ||
		    cursor_kind == CXCursor_ClassTemplatePartialSpecialization)
		{
			template_kind = clang_getTemplateCursorKind(cursor);
			template_metadata(analysis->unit, cursor, value);
		}
		value.kind = template_kind == CXCursor_StructDecl ? "struct" : "class";
		if (cursor_kind == CXCursor_EnumDecl) enum_metadata(analysis->unit, cursor, value);
		value.is_definition = clang_isCursorDefinition(cursor) != 0;
		value.is_specialization = cursor_kind == CXCursor_ClassTemplatePartialSpecialization ||
		                          !clang_Cursor_isNull(clang_getSpecializedCursorTemplate(cursor));
		if (!value.file.empty() && !value.name.empty())
		{
			analysis->declarations.push_back(std::move(value));
		}
		return CXChildVisit_Recurse;
	}

	if (cursor_kind == CXCursor_FunctionDecl)
	{
		hard_function value;
		CXFile file = nullptr;
		clang_getExpansionLocation(
		        clang_getCursorLocation(cursor),
		        &file,
		        nullptr,
		        nullptr,
		        nullptr);
		value.file = file_name(file);
		value.name = to_string(clang_getCursorSpelling(cursor));
		value.is_definition = clang_isCursorDefinition(cursor) != 0;
		value.is_global = is_global_function(cursor);
		if (!value.file.empty() && !value.name.empty())
		{
			analysis->functions.push_back(std::move(value));
		}
	}
	return CXChildVisit_Recurse;
}

void collect_diagnostics(hard_clang_analysis* analysis)
{
	unsigned count = clang_getNumDiagnostics(analysis->unit);
	analysis->diagnostics.reserve(count);
	for (unsigned index = 0; index < count; ++index)
	{
		CXDiagnostic diagnostic = clang_getDiagnostic(analysis->unit, index);
		hard_diagnostic value;
		value.severity = clang_getDiagnosticSeverity(diagnostic);
		value.text = to_string(clang_formatDiagnostic(
		        diagnostic,
		        clang_defaultDiagnosticDisplayOptions()));
		value.category = to_string(clang_getDiagnosticCategoryText(diagnostic));
		CXFile file = nullptr;
		clang_getExpansionLocation(
		        clang_getDiagnosticLocation(diagnostic),
		        &file,
		        &value.line,
		        &value.column,
		        nullptr);
		value.file = file_name(file);
		analysis->diagnostics.push_back(std::move(value));
		clang_disposeDiagnostic(diagnostic);
	}
}

const char* parse_error(CXErrorCode code)
{
	switch (code)
	{
		case CXError_Success:
			return "";
		case CXError_Failure:
			return "libclang failed to parse the translation unit";
		case CXError_Crashed:
			return "libclang crashed while parsing the translation unit";
		case CXError_InvalidArguments:
			return "libclang rejected the translation-unit arguments";
		case CXError_ASTReadError:
			return "libclang could not deserialize the translation unit";
	}
	return "libclang returned an unknown parse error";
}

} // namespace

extern "C" const char* hard_clang_version()
{
	static const std::string version = to_string(clang_getClangVersion());
	return version.c_str();
}

extern "C" hard_clang_analysis* hard_clang_analyze(
        const char* source,
        const char* contents,
        const char* const* arguments,
        int argument_count,
        int skip_function_bodies,
        int* error_code)
{
	auto* analysis = new hard_clang_analysis;
	analysis->index = clang_createIndex(0, 0);
	if (!analysis->index)
	{
		analysis->error = "libclang could not create an index";
		if (error_code)
		{
			*error_code = CXError_Failure;
		}
		return analysis;
	}

	CXUnsavedFile unsaved;
	CXUnsavedFile* unsaved_files = nullptr;
	unsigned unsaved_count = 0;
	if (contents)
	{
		unsaved.Filename = source;
		unsaved.Contents = contents;
		unsaved.Length = static_cast<unsigned long>(std::char_traits<char>::length(contents));
		unsaved_files = &unsaved;
		unsaved_count = 1;
	}

	unsigned options = CXTranslationUnit_DetailedPreprocessingRecord |
	                   CXTranslationUnit_KeepGoing;
	if (skip_function_bodies)
	{
		options |= CXTranslationUnit_SkipFunctionBodies;
	}
	++parse_count;
	CXErrorCode code = clang_parseTranslationUnit2(
	        analysis->index,
	        source,
	        arguments,
	        argument_count,
	        unsaved_files,
	        unsaved_count,
	        options,
	        &analysis->unit);
	if (error_code)
	{
		*error_code = code;
	}
	if (code != CXError_Success || !analysis->unit)
	{
		analysis->error = parse_error(code);
		return analysis;
	}

	collect_diagnostics(analysis);
	clang_visitChildren(
	        clang_getTranslationUnitCursor(analysis->unit),
	        visit_cursor,
	        analysis);
	return analysis;
}

extern "C" void hard_clang_analysis_dispose(hard_clang_analysis* analysis)
{
	if (!analysis)
	{
		return;
	}
	if (analysis->unit)
	{
		clang_disposeTranslationUnit(analysis->unit);
	}
	if (analysis->index)
	{
		clang_disposeIndex(analysis->index);
	}
	delete analysis;
}

extern "C" unsigned long long hard_clang_parse_count()
{
	return parse_count.load();
}

extern "C" const char* hard_clang_analysis_error(const hard_clang_analysis* analysis)
{
	return analysis ? analysis->error.c_str() : "libclang returned no analysis";
}

extern "C" size_t hard_clang_include_count(const hard_clang_analysis* analysis)
{
	return analysis ? analysis->includes.size() : 0;
}

extern "C" const char* hard_clang_include_source(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->includes[index].source.c_str();
}

extern "C" const char* hard_clang_include_target(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->includes[index].target.c_str();
}

extern "C" const char* hard_clang_include_spelling(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->includes[index].spelling.c_str();
}

extern "C" int hard_clang_include_is_system(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->includes[index].is_system;
}

extern "C" size_t hard_clang_declaration_count(const hard_clang_analysis* analysis)
{
	return analysis ? analysis->declarations.size() : 0;
}

extern "C" const char* hard_clang_declaration_file(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].file.c_str();
}

extern "C" const char* hard_clang_declaration_name(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].name.c_str();
}

extern "C" const char* hard_clang_declaration_kind(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].kind.c_str();
}

extern "C" int hard_clang_declaration_is_definition(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].is_definition;
}

extern "C" const char* hard_clang_declaration_identity(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].identity.c_str();
}

extern "C" const char* hard_clang_declaration_enum_base(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].enum_base.c_str();
}

extern "C" const char* hard_clang_declaration_constraint(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].constraint.c_str();
}

extern "C" const char* hard_clang_declaration_unsupported(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].unsupported.c_str();
}

extern "C" size_t hard_clang_declaration_requirement_count(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].requirements.size();
}

extern "C" const char* hard_clang_declaration_requirement(const hard_clang_analysis* analysis, size_t index, size_t requirement_index)
{
	return analysis->declarations[index].requirements[requirement_index].c_str();
}

extern "C" int hard_clang_declaration_is_specialization(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].is_specialization;
}

extern "C" unsigned hard_clang_declaration_offset(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].offset;
}

extern "C" size_t hard_clang_declaration_namespace_count(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->declarations[index].namespaces.size();
}

extern "C" const char* hard_clang_declaration_namespace_name(
        const hard_clang_analysis* analysis,
        size_t declaration_index,
        size_t namespace_index)
{
	return analysis->declarations[declaration_index].namespaces[namespace_index].name.c_str();
}

extern "C" int hard_clang_declaration_namespace_is_inline(
        const hard_clang_analysis* analysis,
        size_t declaration_index,
        size_t namespace_index)
{
	return analysis->declarations[declaration_index].namespaces[namespace_index].is_inline;
}

extern "C" size_t hard_clang_declaration_template_parameter_count(
        const hard_clang_analysis* analysis,
        size_t index)
{
	return analysis->declarations[index].template_parameters.size();
}

extern "C" const char* hard_clang_declaration_template_parameter(
        const hard_clang_analysis* analysis,
        size_t declaration_index,
        size_t parameter_index)
{
	return analysis->declarations[declaration_index].template_parameters[parameter_index].c_str();
}

extern "C" size_t hard_clang_function_count(const hard_clang_analysis* analysis)
{
	return analysis ? analysis->functions.size() : 0;
}

extern "C" const char* hard_clang_function_file(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->functions[index].file.c_str();
}

extern "C" const char* hard_clang_function_name(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->functions[index].name.c_str();
}

extern "C" int hard_clang_function_is_definition(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->functions[index].is_definition;
}

extern "C" int hard_clang_function_is_global(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->functions[index].is_global;
}

extern "C" size_t hard_clang_diagnostic_count(const hard_clang_analysis* analysis)
{
	return analysis ? analysis->diagnostics.size() : 0;
}

extern "C" unsigned hard_clang_diagnostic_severity(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->diagnostics[index].severity;
}

extern "C" const char* hard_clang_diagnostic_text(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->diagnostics[index].text.c_str();
}

extern "C" const char* hard_clang_diagnostic_category(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->diagnostics[index].category.c_str();
}

extern "C" const char* hard_clang_diagnostic_file(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->diagnostics[index].file.c_str();
}

extern "C" unsigned hard_clang_diagnostic_line(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->diagnostics[index].line;
}

extern "C" unsigned hard_clang_diagnostic_column(const hard_clang_analysis* analysis, size_t index)
{
	return analysis->diagnostics[index].column;
}
