#pragma once

#include <string>

namespace fixtures
{

class greeting
{
public:
	explicit greeting(std::string name);

	std::string message() const;

private:
	std::string name_;
};

}
