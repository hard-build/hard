#include "greeting.h"

#include <utility>

fixtures::greeting::greeting(std::string name) :
        name_(std::move(name))
{
}

std::string fixtures::greeting::message() const
{
	return "hello " + name_;
}
