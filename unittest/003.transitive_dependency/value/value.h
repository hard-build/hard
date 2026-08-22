#pragma once

namespace fixtures
{

class value
{
public:
	explicit value(int number);

	int get() const;

private:
	int number_;
};

}
