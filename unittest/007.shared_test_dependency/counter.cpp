#include "counter.h"

void fixtures::counter::increment()
{
	++value_;
}

void fixtures::counter::reset()
{
	value_ = 0;
}

int fixtures::counter::value() const
{
	return value_;
}
