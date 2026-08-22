#include "calculator.h"

#include <stdexcept>

int fixtures::calculator::add(int left, int right) const
{
	return left + right;
}

int fixtures::calculator::subtract(int left, int right) const
{
	return left - right;
}

int fixtures::calculator::multiply(int left, int right) const
{
	return left * right;
}

int fixtures::calculator::divide(int left, int right) const
{
	if (right == 0)
	{
		throw std::invalid_argument("division by zero");
	}
	return left / right;
}
