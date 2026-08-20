#pragma once

#include <stddef.h>

#ifdef __cplusplus
extern "C"
{
#endif

	typedef struct hard_clang_analysis hard_clang_analysis;

	hard_clang_analysis* hard_clang_analyze(
	        const char* source,
	        const char* contents,
	        const char* const* arguments,
	        int argument_count,
	        int skip_function_bodies,
	        int* error_code);

	void hard_clang_analysis_dispose(hard_clang_analysis* analysis);

	const char* hard_clang_analysis_error(const hard_clang_analysis* analysis);

	size_t hard_clang_include_count(const hard_clang_analysis* analysis);
	const char* hard_clang_include_source(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_include_target(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_include_spelling(const hard_clang_analysis* analysis, size_t index);
	int hard_clang_include_is_system(const hard_clang_analysis* analysis, size_t index);

	size_t hard_clang_declaration_count(const hard_clang_analysis* analysis);
	const char* hard_clang_declaration_file(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_declaration_name(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_declaration_kind(const hard_clang_analysis* analysis, size_t index);
	int hard_clang_declaration_is_definition(const hard_clang_analysis* analysis, size_t index);
	int hard_clang_declaration_is_specialization(const hard_clang_analysis* analysis, size_t index);
	unsigned hard_clang_declaration_offset(const hard_clang_analysis* analysis, size_t index);
	size_t hard_clang_declaration_namespace_count(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_declaration_namespace_name(
	        const hard_clang_analysis* analysis,
	        size_t declaration_index,
	        size_t namespace_index);
	int hard_clang_declaration_namespace_is_inline(
	        const hard_clang_analysis* analysis,
	        size_t declaration_index,
	        size_t namespace_index);
	size_t hard_clang_declaration_template_parameter_count(
	        const hard_clang_analysis* analysis,
	        size_t index);
	const char* hard_clang_declaration_template_parameter(
	        const hard_clang_analysis* analysis,
	        size_t declaration_index,
	        size_t parameter_index);

	size_t hard_clang_function_count(const hard_clang_analysis* analysis);
	const char* hard_clang_function_file(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_function_name(const hard_clang_analysis* analysis, size_t index);
	int hard_clang_function_is_definition(const hard_clang_analysis* analysis, size_t index);
	int hard_clang_function_is_global(const hard_clang_analysis* analysis, size_t index);

	size_t hard_clang_diagnostic_count(const hard_clang_analysis* analysis);
	unsigned hard_clang_diagnostic_severity(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_diagnostic_text(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_diagnostic_category(const hard_clang_analysis* analysis, size_t index);
	const char* hard_clang_diagnostic_file(const hard_clang_analysis* analysis, size_t index);
	unsigned hard_clang_diagnostic_line(const hard_clang_analysis* analysis, size_t index);
	unsigned hard_clang_diagnostic_column(const hard_clang_analysis* analysis, size_t index);

#ifdef __cplusplus
}
#endif
