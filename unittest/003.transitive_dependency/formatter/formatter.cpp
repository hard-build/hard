#include "formatter.h"

std::string fixtures::formatter::render(const value& input) const
{
	return "value=" + std::to_string(input.get());
}
