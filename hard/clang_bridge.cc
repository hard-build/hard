#include "clang_bridge.h"

#include <algorithm>
#include <string>
#include <utility>
#include <vector>

#include <clang-c/Index.h>

namespace
{

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

std::vector<std::string> template_parameters(CXTranslationUnit unit, CXCursor cursor)
{
	CXToken* tokens = nullptr;
	unsigned token_count = 0;
	clang_tokenize(unit, clang_getCursorExtent(cursor), &tokens, &token_count);
	std::vector<std::string> parameters;
	std::string parameter;
	unsigned depth = 0;
	bool started = false;
	for (unsigned index = 0; index < token_count; ++index)
	{
		std::string token = to_string(clang_getTokenSpelling(unit, tokens[index]));
		bool finished = false;
		if (!started)
		{
			if (token == "<")
			{
				started = true;
				depth = 1;
			}
			continue;
		}
		if (token == "<")
		{
			++depth;
		}
		else if (!token.empty() && token.find_first_not_of('>') == std::string::npos)
		{
			std::string retained;
			for (char character : token)
			{
				(void)character;
				if (--depth == 0)
				{
					finished = true;
					break;
				}
				retained += '>';
			}
			token = std::move(retained);
		}
		else if (token == "," && depth == 1)
		{
			if (!parameter.empty())
			{
				parameters.push_back(std::move(parameter));
				parameter.clear();
			}
			continue;
		}
		if (!token.empty() && !parameter.empty())
		{
			parameter += ' ';
		}
		parameter += token;
		if (finished)
		{
			if (!parameter.empty())
			{
				parameters.push_back(std::move(parameter));
			}
			break;
		}
	}
	clang_disposeTokens(unit, tokens, token_count);
	return parameters;
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
	                   cursor_kind == CXCursor_StructDecl ||
	                   cursor_kind == CXCursor_ClassTemplate ||
	                   cursor_kind == CXCursor_ClassTemplatePartialSpecialization;
	if (declaration)
	{
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
		CXCursorKind template_kind = cursor_kind;
		if (cursor_kind == CXCursor_ClassTemplate ||
		    cursor_kind == CXCursor_ClassTemplatePartialSpecialization)
		{
			template_kind = clang_getTemplateCursorKind(cursor);
			value.template_parameters = template_parameters(analysis->unit, cursor);
		}
		value.kind = template_kind == CXCursor_StructDecl ? "struct" : "class";
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
