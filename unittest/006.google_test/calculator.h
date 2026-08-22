#pragma once

namespace fixtures
{

class calculator
{
public:
	int add(int left, int right) const;
	int subtract(int left, int right) const;
	int multiply(int left, int right) const;
	int divide(int left, int right) const;
};

}
