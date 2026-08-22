#pragma once

#include <string>

#include "../value/value.h"

namespace fixtures
{

class formatter
{
public:
	std::string render(const value& input) const;
};

}
