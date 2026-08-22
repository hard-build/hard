#pragma once

namespace fixtures
{

class counter
{
public:
	void increment();
	void reset();
	int value() const;

private:
	int value_ = 0;
};

}
