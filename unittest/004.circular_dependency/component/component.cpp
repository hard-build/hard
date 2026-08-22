#include "component.h"

fixtures::component::component(int value) :
        value_(value)
{
}

int fixtures::component::value() const
{
	return value_;
}
