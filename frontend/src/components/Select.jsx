import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import ReactSelect from 'react-select';

import 'components/Select.scss';

class Select extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  onChange(option) {
    if (this.props.onChange) {
      this.props.onChange(option.value);
    }
  }

  render() {
    const { value, options } = this.props;
    const option = options.find(option => option.value == value);
    return (
      <ReactSelect
        className={`Select ${this.props.palette}`}
        classNamePrefix="Select" 
        isSearchable={this.props.isSearchable} 
        options={this.props.options}
        onChange={this.onChange}
        value={option}
        placeholder={this.props.placeholder}
        inputId={this.props.id || `select-${Date.now()}-${Math.floor(Math.random()*1000)}`}
        formatOptionLabel={this.props.optionRenderer}
      />
    );
  }
}

Select.defaultProps = {
  options: [],
  isSearchable: false,
  palette: 'dark',
  placeholder: 'Select...'
};

Select.propTypes = {
  id: PropTypes.string,
  options: PropTypes.arrayOf(PropTypes.shape({
    value: PropTypes.string,
    label: PropTypes.string
  })).isRequired,
  isSearchable: PropTypes.bool,
  onChange: PropTypes.func,
  value: PropTypes.string,
  placeholder: PropTypes.string,
  palette: PropTypes.string,
  optionRenderer: PropTypes.func
};

export default Select;
